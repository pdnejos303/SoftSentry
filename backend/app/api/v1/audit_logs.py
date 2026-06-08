"""Audit log viewer endpoints (Module 9 — spec 9.6, admin only)."""

from __future__ import annotations

import json
from datetime import datetime
from typing import Annotated

from fastapi import APIRouter, Depends, Query
from fastapi.responses import StreamingResponse

from app.core.deps import DBSession, require_role
from app.models.user import User
from app.schemas.users import AuditLogItem, AuditLogList
from app.services import audit_service
from app.services.csv_export import iter_csv

router = APIRouter()

AdminUser = Annotated[User, Depends(require_role("dev"))]


def _pages(total: int, page_size: int) -> int:
    return (total + page_size - 1) // page_size


@router.get("", response_model=AuditLogList, summary="List audit log entries")
async def list_audit_logs(
    session: DBSession,
    _: AdminUser,
    user_uuid: str | None = None,
    action: str | None = None,
    entity_type: str | None = None,
    date_from: datetime | None = None,
    date_to: datetime | None = None,
    page: Annotated[int, Query(ge=1)] = 1,
    page_size: Annotated[int, Query(ge=1, le=200)] = 50,
) -> AuditLogList:
    items, total = await audit_service.list_audit_logs(
        session,
        user_uuid=user_uuid,
        action=action,
        entity_type=entity_type,
        date_from=date_from,
        date_to=date_to,
        page=page,
        page_size=page_size,
    )
    return AuditLogList(
        items=[AuditLogItem.model_validate(i) for i in items],
        total=total,
        page=page,
        page_size=page_size,
        total_pages=_pages(total, page_size),
    )


@router.get("/actions", summary="Distinct action values (for the filter dropdown)")
async def audit_actions(session: DBSession, _: AdminUser) -> list[str]:
    return await audit_service.distinct_actions(session)


@router.get("/export", summary="Export audit log as CSV")
async def export_audit_logs(
    session: DBSession,
    _: AdminUser,
    user_uuid: str | None = None,
    action: str | None = None,
    entity_type: str | None = None,
    date_from: datetime | None = None,
    date_to: datetime | None = None,
) -> StreamingResponse:
    items, _total = await audit_service.list_audit_logs(
        session,
        user_uuid=user_uuid,
        action=action,
        entity_type=entity_type,
        date_from=date_from,
        date_to=date_to,
        page=1,
        page_size=50000,
    )
    header = [
        "created_at",
        "actor_email",
        "action",
        "entity_type",
        "entity_id",
        "ip_address",
        "changes",
        "user_agent",
    ]
    rows = (
        [
            i["created_at"],
            i["actor_email"],
            i["action"],
            i["entity_type"],
            i["entity_id"],
            i["ip_address"],
            json.dumps(i["changes"], ensure_ascii=False) if i["changes"] else "",
            i["user_agent"],
        ]
        for i in items
    )
    return StreamingResponse(
        iter_csv(header, rows),
        media_type="text/csv; charset=utf-8",
        headers={"Content-Disposition": 'attachment; filename="audit-logs.csv"'},
    )
