"""Report generation, listing, download + schedule CRUD (Module 8).

Async generation: POST enqueues an arq job and returns the report uuid; clients
poll GET until status == completed, then download. Literal sub-paths
(`/generate`, `/schedules...`) are declared before `/{report_uuid}` so they win
the route match.
"""

from __future__ import annotations

import uuid as uuid_lib
from typing import Annotated

from fastapi import APIRouter, Depends, HTTPException, Query, Request, status
from fastapi.responses import FileResponse

from app.api.v1._audit import audit
from app.core.deps import CurrentUser, DBSession, require_role
from app.models.report import STATUS_COMPLETED, ReportSchedule
from app.models.user import User
from app.schemas.reports import (
    ReportGenerateRequest,
    ReportList,
    ReportOut,
    ReportQueued,
    ScheduleIn,
    ScheduleList,
    ScheduleOut,
    ScheduleUpdate,
)
from app.services import cron_service, report_service, schedule_service
from app.workers.report_jobs import enqueue_report

router = APIRouter()

# The acting admin, bound (not discarded) so mutations can be audit-attributed.
AdminUser = Annotated[User, Depends(require_role("dev"))]


# ── Generation + listing ─────────────────────────────────────────────────────


@router.post(
    "/generate",
    response_model=ReportQueued,
    status_code=status.HTTP_202_ACCEPTED,
    summary="Queue a report for generation",
)
async def generate_report(
    payload: ReportGenerateRequest,
    session: DBSession,
    user: CurrentUser,
    request: Request,
) -> ReportQueued:
    report = await report_service.create_report(
        session,
        type=payload.type,
        format=payload.format,
        machine_uuid=payload.machine_uuid,
        params=payload.params,
        user_id=user.id,
    )
    await audit(
        session, request, user,
        action="report.generate", entity_type="report", entity_id=str(report.uuid),
        after={"type": payload.type, "format": payload.format},
    )
    await session.commit()
    await enqueue_report(report.uuid)
    return ReportQueued(uuid=report.uuid, status=report.status)


@router.get("", response_model=ReportList, summary="List reports")
async def list_reports(
    session: DBSession,
    _: CurrentUser,
    type: str | None = None,
    page: Annotated[int, Query(ge=1)] = 1,
    page_size: Annotated[int, Query(ge=1, le=100)] = 50,
) -> ReportList:
    reports, total = await report_service.list_reports(
        session, type=type, page=page, page_size=page_size
    )
    items = [ReportOut(**await report_service.to_dict(session, r)) for r in reports]
    total_pages = (total + page_size - 1) // page_size
    return ReportList(
        items=items, total=total, page=page, page_size=page_size, total_pages=total_pages
    )


# ── Schedules (declared before /{report_uuid}) ───────────────────────────────


@router.get("/schedules", response_model=ScheduleList, summary="List report schedules")
async def list_schedules(session: DBSession, _: CurrentUser) -> ScheduleList:
    rows = await schedule_service.list_schedules(session)
    items = [ScheduleOut(**schedule_service.to_dict(s)) for s in rows]
    return ScheduleList(items=items, total=len(items))


@router.post(
    "/schedules",
    response_model=ScheduleOut,
    status_code=status.HTTP_201_CREATED,
    summary="Admin: create a report schedule",
)
async def create_schedule(
    payload: ScheduleIn,
    session: DBSession,
    admin: AdminUser,
    request: Request,
) -> ScheduleOut:
    try:
        sched = await schedule_service.create_schedule(session, payload, user_id=admin.id)
    except cron_service.InvalidCron as exc:
        raise HTTPException(status.HTTP_422_UNPROCESSABLE_ENTITY, str(exc)) from exc
    await audit(
        session, request, admin,
        action="schedule.create", entity_type="report_schedule", entity_id=str(sched.uuid),
        after={"type": sched.type, "format": sched.format, "cron": sched.cron},
    )
    await session.commit()
    return ScheduleOut(**schedule_service.to_dict(sched))


async def _get_schedule_or_404(
    session: DBSession, schedule_uuid: uuid_lib.UUID
) -> ReportSchedule:
    sched = await schedule_service.get_schedule(session, schedule_uuid)
    if sched is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "Schedule not found")
    return sched


@router.patch(
    "/schedules/{schedule_uuid}", response_model=ScheduleOut, summary="Admin: edit a schedule"
)
async def update_schedule(
    schedule_uuid: uuid_lib.UUID,
    payload: ScheduleUpdate,
    session: DBSession,
    admin: AdminUser,
    request: Request,
) -> ScheduleOut:
    sched = await _get_schedule_or_404(session, schedule_uuid)
    try:
        schedule_service.update_schedule(sched, payload)
    except cron_service.InvalidCron as exc:
        raise HTTPException(status.HTTP_422_UNPROCESSABLE_ENTITY, str(exc)) from exc
    await audit(
        session, request, admin,
        action="schedule.update", entity_type="report_schedule", entity_id=str(schedule_uuid),
        after=payload.model_dump(exclude_unset=True, mode="json"),
    )
    await session.commit()
    return ScheduleOut(**schedule_service.to_dict(sched))


@router.delete(
    "/schedules/{schedule_uuid}",
    status_code=status.HTTP_204_NO_CONTENT,
    summary="Admin: delete a schedule",
)
async def delete_schedule(
    schedule_uuid: uuid_lib.UUID, session: DBSession, admin: AdminUser, request: Request
) -> None:
    sched = await _get_schedule_or_404(session, schedule_uuid)
    await schedule_service.delete_schedule(session, sched)
    await audit(
        session, request, admin,
        action="schedule.delete", entity_type="report_schedule", entity_id=str(schedule_uuid),
        before={"name": sched.name, "type": sched.type, "cron": sched.cron},
    )
    await session.commit()


@router.post(
    "/schedules/{schedule_uuid}/run-now",
    response_model=ReportQueued,
    status_code=status.HTTP_202_ACCEPTED,
    summary="Admin: trigger a scheduled report now",
)
async def run_schedule_now(
    schedule_uuid: uuid_lib.UUID, session: DBSession, admin: AdminUser, request: Request
) -> ReportQueued:
    sched = await _get_schedule_or_404(session, schedule_uuid)
    report = await report_service.create_report(
        session,
        type=sched.type,
        format=sched.format,
        machine_uuid=schedule_service.machine_uuid_of(sched.params),
        params={},
        user_id=None,
        schedule_id=sched.id,
    )
    await audit(
        session, request, admin,
        action="schedule.run_now", entity_type="report_schedule", entity_id=str(schedule_uuid),
        after={"report_uuid": str(report.uuid)},
    )
    await session.commit()
    await enqueue_report(report.uuid)
    return ReportQueued(uuid=report.uuid, status=report.status)


# ── Single report (/{report_uuid}) ───────────────────────────────────────────


@router.get("/{report_uuid}", response_model=ReportOut, summary="Report status / metadata")
async def get_report(
    report_uuid: uuid_lib.UUID, session: DBSession, _: CurrentUser
) -> ReportOut:
    report = await report_service.get_report(session, report_uuid)
    if report is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "Report not found")
    return ReportOut(**await report_service.to_dict(session, report))


@router.get("/{report_uuid}/download", summary="Download a completed report")
async def download_report(
    report_uuid: uuid_lib.UUID, session: DBSession, _: CurrentUser
) -> FileResponse:
    report = await report_service.get_report(session, report_uuid)
    if report is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "Report not found")
    if report.status != STATUS_COMPLETED or not report.file_path:
        raise HTTPException(status.HTTP_409_CONFLICT, "Report is not ready for download")
    if not report_service.file_exists(report.file_path):
        raise HTTPException(status.HTTP_410_GONE, "Report file no longer exists")
    media = "application/pdf" if report.format == "pdf" else "text/csv; charset=utf-8"
    return FileResponse(
        report.file_path,
        media_type=media,
        filename=f"{report.type}-{report.uuid}.{report.format}",
    )


@router.delete(
    "/{report_uuid}",
    status_code=status.HTTP_204_NO_CONTENT,
    summary="Admin: delete a report",
)
async def delete_report(
    report_uuid: uuid_lib.UUID, session: DBSession, admin: AdminUser, request: Request
) -> None:
    report = await report_service.get_report(session, report_uuid)
    if report is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "Report not found")
    await audit(
        session, request, admin,
        action="report.delete", entity_type="report", entity_id=str(report_uuid),
        before={"type": report.type, "format": report.format, "status": report.status},
    )
    await report_service.delete_report(session, report)
    await session.commit()
