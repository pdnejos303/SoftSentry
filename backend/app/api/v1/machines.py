"""Dashboard-facing machine endpoints — list, detail, tag edit, delete, trigger-scan."""

from __future__ import annotations

import uuid as uuid_lib
from datetime import UTC, datetime
from typing import Annotated

from fastapi import APIRouter, Depends, HTTPException, Query, status

from app.core.config import settings
from app.core.deps import CurrentUser, DBSession, require_role
from app.models.machine import Machine
from app.schemas.dashboard import RiskBreakdown
from app.schemas.machines import (
    MachineDetail,
    MachineList,
    MachineUpdate,
    TriggerScanResponse,
    TriggerUpdateResponse,
)
from app.schemas.software import (
    MachineSoftwareList,
    ScanHistoryList,
    SoftwareHistoryList,
)
from app.schemas.vulnerability import VulnerabilityItem, VulnerabilityList
from app.services import (
    binary_service,
    machine_service,
    risk_service,
    software_service,
    vulnerability_service,
)

router = APIRouter()


@router.get("", response_model=MachineList, summary="List machines")
async def list_machines(
    session: DBSession,
    _: CurrentUser,
    q: str | None = None,
    status_filter: Annotated[str | None, Query(alias="status")] = None,
    os: str | None = None,
    tag: str | None = None,
    page: Annotated[int, Query(ge=1)] = 1,
    page_size: Annotated[int, Query(ge=1, le=200)] = 50,
    sort: str | None = None,
) -> MachineList:
    items, total = await machine_service.list_machines(
        session,
        q=q,
        status=status_filter,
        os=os,
        tag=tag,
        page=page,
        page_size=page_size,
        sort=sort,
    )
    total_pages = (total + page_size - 1) // page_size
    return MachineList(
        items=items,
        total=total,
        page=page,
        page_size=page_size,
        total_pages=total_pages,
    )


async def _get_or_404(session: DBSession, machine_uuid: uuid_lib.UUID) -> Machine:
    machine = await machine_service.get_machine(session, machine_uuid)
    if machine is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "Machine not found")
    return machine


@router.get("/{machine_uuid}", response_model=MachineDetail, summary="Machine detail")
async def get_machine(
    machine_uuid: uuid_lib.UUID,
    session: DBSession,
    _: CurrentUser,
) -> MachineDetail:
    machine = await _get_or_404(session, machine_uuid)
    return MachineDetail(**await machine_service.machine_detail(session, machine))


@router.patch(
    "/{machine_uuid}",
    response_model=MachineDetail,
    summary="Admin: edit machine name/owner/tags",
    dependencies=[Depends(require_role("dev", "admin"))],
)
async def update_machine(
    machine_uuid: uuid_lib.UUID,
    payload: MachineUpdate,
    session: DBSession,
) -> MachineDetail:
    machine = await _get_or_404(session, machine_uuid)
    # Partial update: only apply fields the client actually sent, so editing the
    # name doesn't wipe tags and vice versa.
    fields = payload.model_dump(exclude_unset=True)
    if "tags" in fields and fields["tags"] is not None:
        machine.tags = fields["tags"]
    if "display_name" in fields:
        machine.display_name = fields["display_name"]
    if "owner" in fields:
        machine.owner = fields["owner"]
    await session.commit()
    return MachineDetail(**await machine_service.machine_detail(session, machine))


@router.delete(
    "/{machine_uuid}",
    status_code=status.HTTP_204_NO_CONTENT,
    summary="Admin: soft-delete a machine",
    dependencies=[Depends(require_role("dev", "admin"))],
)
async def delete_machine(
    machine_uuid: uuid_lib.UUID,
    session: DBSession,
) -> None:
    machine = await _get_or_404(session, machine_uuid)
    machine.deleted_at = datetime.now(tz=UTC)
    await session.commit()


@router.post(
    "/{machine_uuid}/trigger-scan",
    response_model=TriggerScanResponse,
    status_code=status.HTTP_202_ACCEPTED,
    summary="Admin: request a scan on the agent's next heartbeat",
    dependencies=[Depends(require_role("dev", "admin"))],
)
async def trigger_scan(
    machine_uuid: uuid_lib.UUID,
    session: DBSession,
) -> TriggerScanResponse:
    machine = await _get_or_404(session, machine_uuid)
    await machine_service.request_scan(session, machine)
    await session.commit()
    return TriggerScanResponse(manual_scan_requested=True)


@router.post(
    "/{machine_uuid}/trigger-update",
    response_model=TriggerUpdateResponse,
    status_code=status.HTTP_202_ACCEPTED,
    summary="Admin: force the agent to pull the latest binary on its next heartbeat",
    dependencies=[Depends(require_role("dev", "admin"))],
)
async def trigger_update(
    machine_uuid: uuid_lib.UUID,
    session: DBSession,
) -> TriggerUpdateResponse:
    machine = await _get_or_404(session, machine_uuid)
    # Fail fast if there is nothing to push: without a manifest binary for this
    # machine's platform the forced flag would just be a no-op the admin can't see.
    if binary_service.latest_for(settings.agent_binary_dir, machine.os, machine.arch) is None:
        raise HTTPException(
            status.HTTP_409_CONFLICT,
            f"No agent binary available for {machine.os}/{machine.arch}",
        )
    await machine_service.request_update(session, machine)
    await session.commit()
    return TriggerUpdateResponse(force_update_requested=True)


@router.get(
    "/{machine_uuid}/risk",
    response_model=RiskBreakdown,
    summary="Risk-score breakdown for a machine (spec 7.2)",
)
async def machine_risk(
    machine_uuid: uuid_lib.UUID,
    session: DBSession,
    _: CurrentUser,
) -> RiskBreakdown:
    machine = await _get_or_404(session, machine_uuid)
    breakdown = await risk_service.risk_breakdown(session, machine)
    return RiskBreakdown(
        risk_score=machine.risk_score,
        color=risk_service.risk_color(machine.risk_score),
        **breakdown,
    )


@router.get(
    "/{machine_uuid}/software",
    response_model=MachineSoftwareList,
    summary="Software inventory of a machine",
)
async def machine_software(
    machine_uuid: uuid_lib.UUID,
    session: DBSession,
    _: CurrentUser,
    q: str | None = None,
    signature_status: Annotated[str | None, Query(alias="signature_status")] = None,
    include_inactive: bool = False,
    page: Annotated[int, Query(ge=1)] = 1,
    page_size: Annotated[int, Query(ge=1, le=200)] = 50,
) -> MachineSoftwareList:
    machine = await _get_or_404(session, machine_uuid)
    items, total = await software_service.machine_software(
        session,
        machine,
        q=q,
        signature_status=signature_status,
        include_inactive=include_inactive,
        page=page,
        page_size=page_size,
    )
    total_pages = (total + page_size - 1) // page_size
    return MachineSoftwareList(
        items=items, total=total, page=page, page_size=page_size, total_pages=total_pages
    )


@router.get(
    "/{machine_uuid}/history",
    response_model=SoftwareHistoryList,
    summary="Install/uninstall/update history of a machine",
)
async def machine_history(
    machine_uuid: uuid_lib.UUID,
    session: DBSession,
    _: CurrentUser,
    limit: Annotated[int, Query(ge=1, le=500)] = 100,
) -> SoftwareHistoryList:
    machine = await _get_or_404(session, machine_uuid)
    items, total = await software_service.machine_history(session, machine.id, limit=limit)
    return SoftwareHistoryList(items=items, total=total)


@router.get(
    "/{machine_uuid}/vulnerabilities",
    response_model=VulnerabilityList,
    summary="Vulnerabilities affecting a machine",
)
async def machine_vulnerabilities(
    machine_uuid: uuid_lib.UUID,
    session: DBSession,
    _: CurrentUser,
    severity: str | None = None,
    show_dismissed: bool = False,
    page: Annotated[int, Query(ge=1)] = 1,
    page_size: Annotated[int, Query(ge=1, le=200)] = 50,
) -> VulnerabilityList:
    await _get_or_404(session, machine_uuid)
    severities = [s.strip() for s in severity.split(",") if s.strip()] if severity else None
    items, total = await vulnerability_service.list_vulnerabilities(
        session,
        severities=severities,
        machine_uuid=machine_uuid,
        show_dismissed=show_dismissed,
        page=page,
        page_size=page_size,
    )
    total_pages = (total + page_size - 1) // page_size
    return VulnerabilityList(
        items=[VulnerabilityItem(**i) for i in items],
        total=total,
        page=page,
        page_size=page_size,
        total_pages=total_pages,
    )


@router.get(
    "/{machine_uuid}/scans",
    response_model=ScanHistoryList,
    summary="Scan submission history of a machine",
)
async def machine_scans(
    machine_uuid: uuid_lib.UUID,
    session: DBSession,
    _: CurrentUser,
    limit: Annotated[int, Query(ge=1, le=200)] = 50,
) -> ScanHistoryList:
    machine = await _get_or_404(session, machine_uuid)
    items, total = await software_service.machine_scans(session, machine.id, limit=limit)
    return ScanHistoryList(items=items, total=total)
