"""arq tasks for Module 8 reporting.

- generate_report:   render one queued report to disk.
- run_due_schedules: per-minute cron — fire schedules whose next_run_at has
                     passed, enqueueing a generate_report job for each.
- cleanup_reports:   daily — purge reports past their 30-day retention.
"""

from __future__ import annotations

import uuid as uuid_lib
from datetime import UTC, datetime
from typing import Any

from sqlalchemy import select

from app.core.db import SessionLocal
from app.core.logging import logger
from app.models.report import STATUS_QUEUED, STATUS_RUNNING, Report
from app.services import report_service, schedule_service


async def generate_report(ctx: dict[str, Any], *, report_uuid: str) -> str:
    async with SessionLocal() as session:
        await report_service.run_generation(uuid_lib.UUID(report_uuid), session)
    return report_uuid


async def run_due_schedules(ctx: dict[str, Any]) -> dict[str, int]:
    now = datetime.now(tz=UTC)
    fired = 0
    skipped = 0
    redis = ctx.get("redis")
    async with SessionLocal() as session:
        due = await schedule_service.due_schedules(session, now)
        for sched in due:
            # Edge 5: previous run still in flight → skip, just roll next_run_at.
            in_flight = (
                await session.execute(
                    select(Report.id).where(
                        Report.schedule_id == sched.id,
                        Report.status.in_([STATUS_QUEUED, STATUS_RUNNING]),
                    )
                )
            ).first()
            if in_flight is not None:
                schedule_service.mark_ran(sched, now)
                skipped += 1
                logger.warning("schedule_skip_overlap", schedule_uuid=str(sched.uuid))
                continue

            report = await report_service.create_report(
                session,
                type=sched.type,
                format=sched.format,
                machine_uuid=schedule_service.machine_uuid_of(sched.params),
                params={},
                user_id=None,
                schedule_id=sched.id,
            )
            schedule_service.mark_ran(sched, now)
            await session.flush()
            if redis is not None:
                await redis.enqueue_job("generate_report", report_uuid=str(report.uuid))
            fired += 1
        await session.commit()
    logger.info("schedules_fired", fired=fired, skipped=skipped)
    return {"fired": fired, "skipped": skipped}


async def cleanup_reports(ctx: dict[str, Any]) -> dict[str, int]:
    now = datetime.now(tz=UTC)
    removed = 0
    async with SessionLocal() as session:
        expired = (
            (
                await session.execute(
                    select(Report).where(
                        Report.expires_at.is_not(None), Report.expires_at < now
                    )
                )
            )
            .scalars()
            .all()
        )
        for report in expired:
            await report_service.delete_report(session, report)
            removed += 1
        await session.commit()
    logger.info("reports_cleanup", removed=removed)
    return {"removed": removed}


async def enqueue_report(report_uuid: uuid_lib.UUID) -> bool:
    """Best-effort enqueue of a generate_report job.

    Enqueueing must never fail the API request: if the queue is unreachable the
    report row simply stays `queued` and a later worker (or a manual retry)
    picks it up. We use a fail-fast Redis config so a down queue doesn't block.
    """
    import dataclasses

    from arq import create_pool

    from app.workers import WorkerSettings

    fast = dataclasses.replace(
        WorkerSettings.redis_settings, conn_retries=0, conn_timeout=2
    )
    try:
        pool = await create_pool(fast)
    except Exception as exc:  # any connection error → leave the report queued
        logger.warning("report_enqueue_unavailable", report_uuid=str(report_uuid), error=str(exc))
        return False
    try:
        await pool.enqueue_job("generate_report", report_uuid=str(report_uuid))
    except Exception as exc:
        logger.warning("report_enqueue_failed", report_uuid=str(report_uuid), error=str(exc))
        return False
    finally:
        await pool.aclose()
    return True
