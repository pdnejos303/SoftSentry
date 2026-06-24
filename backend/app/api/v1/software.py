"""Cross-machine software endpoints (Phase 2 A4).

Aggregate inventory view, top-N widget, and two-machine comparison.
"""

from __future__ import annotations

from datetime import date
from typing import Annotated

from fastapi import APIRouter, HTTPException, Query, status

from app.core.deps import CurrentUser, DBSession
from app.schemas.software import (
    CompareRequest,
    CompareResponse,
    CrossSoftwareList,
    RecentInstallList,
    SoftwareStats,
    TopSoftwareItem,
    SoftwareDetailOut,
    SoftwareMachineList,
)
from app.services import machine_service, software_service

router = APIRouter()


@router.get(
    "/recent-installs",
    response_model=RecentInstallList,
    summary="Recently installed/updated apps fleet-wide, with trust verdict",
)
async def recent_installs(
    session: DBSession,
    _: CurrentUser,
    days: Annotated[int | None, Query(ge=1, le=365)] = 30,
    from_date: Annotated[
        date | None, Query(description="Inclusive start of an explicit calendar range (UTC)")
    ] = None,
    to_date: Annotated[
        date | None, Query(description="Inclusive end of an explicit calendar range (UTC)")
    ] = None,
    all_time: Annotated[
        bool, Query(description="Ignore the time window — newest installs across all history")
    ] = False,
    limit: Annotated[int, Query(ge=1, le=500)] = 100,
) -> RecentInstallList:
    if from_date is not None and to_date is not None and from_date > to_date:
        raise HTTPException(status.HTTP_400_BAD_REQUEST, "from_date must not be after to_date")
    items = await software_service.recent_installs(
        session,
        days=None if all_time else days,
        from_date=from_date,
        to_date=to_date,
        limit=limit,
    )
    return RecentInstallList(items=items, total=len(items))


_SORTS = {"installs", "-installs", "name", "-name"}


@router.get("", response_model=CrossSoftwareList, summary="Cross-machine software list")
async def list_software(
    session: DBSession,
    _: CurrentUser,
    q: str | None = None,
    publisher: str | None = None,
    signature_status: Annotated[str | None, Query(alias="signature_status")] = None,
    sort: Annotated[str, Query(pattern="^-?(installs|name)$")] = "installs",
    page: Annotated[int, Query(ge=1)] = 1,
    page_size: Annotated[int, Query(ge=1, le=200)] = 50,
) -> CrossSoftwareList:
    items, total = await software_service.cross_software(
        session,
        q=q,
        publisher=publisher,
        signature_status=signature_status,
        sort=sort if sort in _SORTS else "installs",
        page=page,
        page_size=page_size,
    )
    total_pages = (total + page_size - 1) // page_size
    return CrossSoftwareList(
        items=items, total=total, page=page, page_size=page_size, total_pages=total_pages
    )


@router.get("/stats", response_model=SoftwareStats, summary="Fleet software posture summary")
async def software_stats(
    session: DBSession,
    _: CurrentUser,
    q: str | None = None,
    publisher: str | None = None,
) -> SoftwareStats:
    stats = await software_service.software_stats(session, q=q, publisher=publisher)
    return SoftwareStats(**stats)


@router.get("/top", response_model=list[TopSoftwareItem], summary="Top-N most-installed software")
async def top_software(
    session: DBSession,
    _: CurrentUser,
    limit: Annotated[int, Query(ge=1, le=100)] = 10,
    risk: Annotated[bool, Query(description="Rank untrustworthy-by-signature apps instead")] = False,
) -> list[TopSoftwareItem]:
    items = await software_service.top_software(session, limit=limit, risk=risk)
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

@router.get("/detail", response_model=SoftwareDetailOut, summary="Software detail by name")
async def software_detail(
    session: DBSession,
    _: CurrentUser,
    name: Annotated[str, Query(min_length=1)],
) -> SoftwareDetailOut:
    detail = await software_service.software_detail(session, name=name)
    if detail is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "Software not found")
    return SoftwareDetailOut(**detail)

@router.get("/detail/machines", response_model=SoftwareMachineList,
            summary="Machines that have this software")
async def software_machines(
    session: DBSession,
    _: CurrentUser,
    name: Annotated[str, Query(min_length=1)],
    page: Annotated[int, Query(ge=1)] = 1,
    page_size: Annotated[int, Query(ge=1, le=200)] = 50,
) -> SoftwareMachineList:
    items, total = await software_service.software_machines(
        session, name=name, page=page, page_size=page_size
    )
    total_pages = (total + page_size - 1) // page_size
    return SoftwareMachineList(
        items=items, total=total, page=page, page_size=page_size, total_pages=total_pages
    )