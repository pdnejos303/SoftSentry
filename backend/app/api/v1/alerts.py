"""Alert listing + acknowledge/resolve (shared across detection modules)."""

from __future__ import annotations

import uuid as uuid_lib
from typing import Annotated

from fastapi import APIRouter, HTTPException, Query, Request, status

from app.api.v1._audit import audit
from app.core.deps import CurrentUser, DBSession
from app.schemas.alerts import AlertList, AlertOut
from app.services import alert_service

router = APIRouter()


@router.get("", response_model=AlertList, summary="List alerts")
async def list_alerts(
    session: DBSession,
    _: CurrentUser,
    type: str | None = None,  # comma-separated alert types
    status_: Annotated[str | None, Query(alias="status")] = None,
    severity: str | None = None,
    machine_uuid: uuid_lib.UUID | None = None,
    page: Annotated[int, Query(ge=1)] = 1,
    page_size: Annotated[int, Query(ge=1, le=200)] = 50,
) -> AlertList:
    types = [t.strip() for t in type.split(",") if t.strip()] if type else None
    items, total = await alert_service.list_alerts(
        session,
        types=types,
        status=status_,
        severity=severity,
        machine_uuid=machine_uuid,
        page=page,
        page_size=page_size,
    )
    total_pages = (total + page_size - 1) // page_size
    return AlertList(
        items=[AlertOut(**i) for i in items],
        total=total,
        page=page,
        page_size=page_size,
        total_pages=total_pages,
    )


@router.post("/{alert_uuid}/acknowledge", response_model=AlertOut, summary="Acknowledge an alert")
async def acknowledge_alert(
    alert_uuid: uuid_lib.UUID, session: DBSession, user: CurrentUser, request: Request
) -> AlertOut:
    alert = await alert_service.get_alert(session, alert_uuid)
    if alert is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "Alert not found")
    alert_service.acknowledge(alert)
    await audit(
        session, request, user,
        action="alert.acknowledge", entity_type="alert", entity_id=str(alert_uuid),
        after={"type": alert.type, "status": alert.status},
    )
    payload = await alert_service.to_dict(session, alert)
    await session.commit()
    return AlertOut(**payload)


@router.post("/{alert_uuid}/resolve", response_model=AlertOut, summary="Resolve an alert")
async def resolve_alert(
    alert_uuid: uuid_lib.UUID, session: DBSession, user: CurrentUser, request: Request
) -> AlertOut:
    alert = await alert_service.get_alert(session, alert_uuid)
    if alert is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "Alert not found")
    alert_service.resolve(alert)
    await audit(
        session, request, user,
        action="alert.resolve", entity_type="alert", entity_id=str(alert_uuid),
        after={"type": alert.type, "status": alert.status},
    )
    payload = await alert_service.to_dict(session, alert)
    await session.commit()
    return AlertOut(**payload)
