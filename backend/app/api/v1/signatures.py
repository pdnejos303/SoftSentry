"""Dashboard-facing signature endpoints — list, chain detail, fleet stats (Module 3)."""

from __future__ import annotations

import uuid as uuid_lib
from typing import Annotated

from fastapi import APIRouter, HTTPException, Query, status

from app.core.deps import CurrentUser, DBSession
from app.schemas.signature import (
    SignatureDetail,
    SignatureList,
    SignatureListItem,
    SignatureStats,
)
from app.services import signature_service

router = APIRouter()
dashboard_router = APIRouter()


@router.get("", response_model=SignatureList, summary="List software signatures")
async def list_signatures(
    session: DBSession,
    _: CurrentUser,
    status_filter: Annotated[str | None, Query(alias="status")] = None,  # comma-separated
    publisher: str | None = None,
    q: str | None = None,
    page: Annotated[int, Query(ge=1)] = 1,
    page_size: Annotated[int, Query(ge=1, le=200)] = 50,
) -> SignatureList:
    statuses = (
        [s.strip() for s in status_filter.split(",") if s.strip()] if status_filter else None
    )
    items, total = await signature_service.list_signatures(
        session,
        statuses=statuses,
        publisher=publisher,
        q=q,
        page=page,
        page_size=page_size,
    )
    total_pages = (total + page_size - 1) // page_size
    return SignatureList(
        items=[SignatureListItem(**i) for i in items],
        total=total,
        page=page,
        page_size=page_size,
        total_pages=total_pages,
    )


@router.get(
    "/{software_uuid}",
    response_model=SignatureDetail,
    summary="Signature + certificate chain detail",
)
async def signature_detail(
    software_uuid: uuid_lib.UUID, session: DBSession, _: CurrentUser
) -> SignatureDetail:
    detail = await signature_service.signature_detail(session, software_uuid)
    if detail is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "Signature not found")
    return SignatureDetail(**detail)


@dashboard_router.get(
    "/signature-stats",
    response_model=SignatureStats,
    summary="Signature-status distribution across the fleet",
)
async def signature_stats(session: DBSession, _: CurrentUser) -> SignatureStats:
    stats = await signature_service.signature_stats(session)
    return SignatureStats(**stats)
