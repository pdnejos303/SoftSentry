"""arq task: daily CVE sync from NVD (Module 5.1).

Registered on the worker with a daily cron (and callable on demand via the admin
trigger endpoint). Opens its own DB session + httpx client so it is independent
of the request lifecycle.
"""

from __future__ import annotations

from typing import Any

import httpx

from app.core.config import settings
from app.core.db import SessionLocal
from app.core.logging import logger
from app.services import cve_sync as cve_sync_service


async def cve_sync(ctx: dict[str, Any], *, full: bool = False) -> dict[str, int]:
    """Fetch + upsert CVEs. `full=True` does a complete backfill; otherwise an
    incremental window (last 24h of modifications)."""
    last_modified = None if full else cve_sync_service.default_incremental_window()
    logger.info("cve_sync_start", full=full)
    async with SessionLocal() as session, httpx.AsyncClient() as client:
        try:
            totals = await cve_sync_service.sync_nvd(
                session,
                client,
                last_modified=last_modified,
                api_key=settings.nvd_api_key,
            )
        except httpx.HTTPError as exc:
            # Failed sync → retried by the next scheduled run (spec 5.1).
            logger.error("cve_sync_failed", error=str(exc))
            raise
    logger.info("cve_sync_done", **totals)
    return totals


async def enqueue_cve_sync(*, full: bool = False) -> bool:
    """Enqueue a `cve_sync` job on the arq queue. Returns False if the queue is
    unreachable (caller decides how to surface it)."""
    from arq import create_pool

    from app.workers import WorkerSettings

    try:
        pool = await create_pool(WorkerSettings.redis_settings)
    except (OSError, ConnectionError):
        return False
    try:
        await pool.enqueue_job("cve_sync", full=full)
    finally:
        await pool.aclose()
    return True
