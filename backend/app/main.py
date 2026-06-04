"""FastAPI application entry-point."""

from __future__ import annotations

import asyncio
import time
from contextlib import asynccontextmanager, suppress
from typing import TYPE_CHECKING

from fastapi import FastAPI, Request
from fastapi.middleware.cors import CORSMiddleware
from prometheus_client import CONTENT_TYPE_LATEST, generate_latest
from starlette.responses import Response

from app import __version__
from app.api.v1 import api_router
from app.core import metrics
from app.core.config import settings
from app.core.db import SessionLocal
from app.core.logging import configure_logging, logger
from app.services import metrics_service

if TYPE_CHECKING:
    from collections.abc import AsyncIterator, Awaitable, Callable


@asynccontextmanager
async def lifespan(_: FastAPI) -> AsyncIterator[None]:
    configure_logging()
    logger.info("softsentry.startup", env=settings.env, version=__version__)
    metrics_task = asyncio.create_task(
        metrics_service.run_refresh_loop(SessionLocal, settings.metrics_refresh_seconds)
    )
    try:
        yield
    finally:
        metrics_task.cancel()
        with suppress(asyncio.CancelledError):
            await metrics_task
        logger.info("softsentry.shutdown")


app = FastAPI(
    title="SoftSentry API",
    version=__version__,
    docs_url="/docs" if settings.enable_docs else None,
    redoc_url="/redoc" if settings.enable_docs else None,
    openapi_url="/openapi.json" if settings.enable_docs else None,
    lifespan=lifespan,
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=[settings.dashboard_url],
    allow_credentials=True,
    allow_methods=["GET", "POST", "PATCH", "DELETE", "OPTIONS"],
    allow_headers=["*"],
)

@app.middleware("http")
async def prometheus_http_middleware(
    request: Request,
    call_next: Callable[[Request], Awaitable[Response]],
) -> Response:
    # Skip the scrape endpoint so the collector doesn't measure itself.
    if request.url.path == "/metrics":
        return await call_next(request)
    start = time.perf_counter()
    response = await call_next(request)
    elapsed = time.perf_counter() - start
    # Label on the matched route template (e.g. /api/v1/machines/{uuid}) so
    # cardinality is bounded by route count, not by every distinct URL.
    route = request.scope.get("route")
    path = getattr(route, "path", None) or "unmatched"
    metrics.http_requests_total.labels(
        method=request.method, path=path, status=str(response.status_code)
    ).inc()
    metrics.http_request_duration_seconds.labels(method=request.method, path=path).observe(elapsed)
    return response


app.include_router(api_router)


@app.get("/health", tags=["meta"])
async def healthcheck() -> dict[str, str]:
    return {"status": "ok", "version": __version__}


@app.get("/metrics", tags=["meta"], include_in_schema=False)
async def metrics_endpoint() -> Response:
    return Response(generate_latest(), media_type=CONTENT_TYPE_LATEST)
