"""Report-schedule CRUD + due-calculation (spec 8.5).

Cron is validated and the next fire time computed on every write, so the worker
only has to query `is_active AND next_run_at <= now`.
"""

from __future__ import annotations

import uuid as uuid_lib
from datetime import datetime
from typing import Any

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.models.report import ReportSchedule
from app.schemas.reports import ScheduleIn, ScheduleUpdate
from app.services import cron_service


def machine_uuid_of(params: dict[str, Any]) -> uuid_lib.UUID | None:
    raw = params.get("machine_uuid")
    if raw is None:
        return None
    return raw if isinstance(raw, uuid_lib.UUID) else uuid_lib.UUID(str(raw))


async def create_schedule(
    session: AsyncSession, payload: ScheduleIn, *, user_id: int | None
) -> ReportSchedule:
    cron = cron_service.validate(payload.cron)
    params = dict(payload.params)
    if payload.machine_uuid is not None:
        params["machine_uuid"] = str(payload.machine_uuid)

    sched = ReportSchedule(
        name=payload.name,
        type=payload.type,
        format=payload.format,
        params=params,
        cron=cron,
        recipients=payload.recipients,
        is_active=payload.is_active,
        next_run_at=cron_service.next_run(cron) if payload.is_active else None,
        created_by_user_id=user_id,
    )
    session.add(sched)
    await session.flush()
    return sched


async def list_schedules(session: AsyncSession) -> list[ReportSchedule]:
    rows = (
        (await session.execute(select(ReportSchedule).order_by(ReportSchedule.id.desc())))
        .scalars()
        .all()
    )
    return list(rows)


async def get_schedule(
    session: AsyncSession, schedule_uuid: uuid_lib.UUID
) -> ReportSchedule | None:
    return (
        await session.execute(
            select(ReportSchedule).where(ReportSchedule.uuid == schedule_uuid)
        )
    ).scalar_one_or_none()


def update_schedule(sched: ReportSchedule, payload: ScheduleUpdate) -> None:
    data = payload.model_dump(exclude_unset=True)
    if "cron" in data and data["cron"] is not None:
        data["cron"] = cron_service.validate(data["cron"])
    for field, value in data.items():
        setattr(sched, field, value)
    # Recompute the next fire time when timing or activation changed.
    if sched.is_active:
        sched.next_run_at = cron_service.next_run(sched.cron)
    else:
        sched.next_run_at = None


async def delete_schedule(session: AsyncSession, sched: ReportSchedule) -> None:
    await session.delete(sched)


async def due_schedules(session: AsyncSession, now: datetime) -> list[ReportSchedule]:
    rows = (
        (
            await session.execute(
                select(ReportSchedule).where(
                    ReportSchedule.is_active.is_(True),
                    ReportSchedule.next_run_at.is_not(None),
                    ReportSchedule.next_run_at <= now,
                )
            )
        )
        .scalars()
        .all()
    )
    return list(rows)


def mark_ran(sched: ReportSchedule, now: datetime) -> None:
    sched.last_run_at = now
    sched.next_run_at = cron_service.next_run(sched.cron, after=now)


def to_dict(sched: ReportSchedule) -> dict[str, Any]:
    try:
        upcoming = cron_service.next_runs(sched.cron, count=3)
    except cron_service.InvalidCron:
        upcoming = []
    return {
        "uuid": sched.uuid,
        "name": sched.name,
        "type": sched.type,
        "format": sched.format,
        "machine_uuid": machine_uuid_of(sched.params),
        "cron": sched.cron,
        "recipients": sched.recipients,
        "is_active": sched.is_active,
        "last_run_at": sched.last_run_at,
        "next_run_at": sched.next_run_at,
        "upcoming_runs": upcoming,
        "created_at": sched.created_at,
    }
