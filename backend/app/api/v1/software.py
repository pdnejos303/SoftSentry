"""Cross-machine software endpoints (Phase 2 A4).

Aggregate inventory view, top-N widget, and two-machine comparison.
"""

from __future__ import annotations

from typing import Annotated

from fastapi import APIRouter, HTTPException, Query, status

from app.core.deps import CurrentUser, DBSession
from app.schemas.software import (
    CompareRequest,
    CompareResponse,
    CrossSoftwareList,
    TopSoftwareItem,
)
from app.services import machine_service, software_service

router = APIRouter()


@router.get("", response_model=CrossSoftwareList, summary="Cross-machine software list")
async def list_software(
    session: DBSession,
    _: CurrentUser,
    q: str | None = None,
    publisher: str | None = None,
    signature_status: Annotated[str | None, Query(alias="signature_status")] = None,
    page: Annotated[int, Query(ge=1)] = 1,
    page_size: Annotated[int, Query(ge=1, le=200)] = 50,
) -> CrossSoftwareList:
    items, total = await software_service.cross_software(
        session,
        q=q,
        publisher=publisher,
        signature_status=signature_status,
        page=page,
        page_size=page_size,
    )
    total_pages = (total + page_size - 1) // page_size
    return CrossSoftwareList(
        items=items, total=total, page=page, page_size=page_size, total_pages=total_pages
    )


@router.get("/top", response_model=list[TopSoftwareItem], summary="Top-N most-installed software")
async def top_software(
    session: DBSession,
    _: CurrentUser,
    limit: Annotated[int, Query(ge=1, le=100)] = 10,
) -> list[TopSoftwareItem]:
    items = await software_service.top_software(session, limit=limit)
    return [TopSoftwareItem(**i) for i in items]


@router.post("/compare", response_model=CompareResponse, summary="Compare software on two machines")
async def compare_software(
    payload: CompareRequest,
    session: DBSession,
    _: CurrentUser,
) -> CompareResponse:
    machine_a = await machine_service.get_machine(session, payload.machine_uuids[0])
    machine_b = await machine_service.get_machine(session, payload.machine_uuids[1])
    if machine_a is None or machine_b is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "One or both machines not found")

    result = await software_service.compare(session, machine_a, machine_b)
    return CompareResponse(**result)
