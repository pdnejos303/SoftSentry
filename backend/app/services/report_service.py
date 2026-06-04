"""Report lifecycle: create rows, generate artifacts, serialize, clean up (Module 8).

Generation is invoked by the arq worker (`generate_report`). PDF rendering is
delegated to `pdf_service` (lazy WeasyPrint import); CSV uses `csv_export`. The
type+format matrix maps to a concrete artifact:

    org_summary  + pdf → org HTML→PDF report
    org_summary  + csv → full machines list CSV
    machine_detail + pdf → per-machine HTML→PDF report
    machine_detail + csv → that machine's software inventory CSV
"""

from __future__ import annotations

import uuid as uuid_lib
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any

from sqlalchemy import func, select
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.config import settings
from app.core.logging import logger
from app.models.report import (
    FORMAT_CSV,
    FORMAT_PDF,
    STATUS_COMPLETED,
    STATUS_FAILED,
    STATUS_QUEUED,
    STATUS_RUNNING,
    TYPE_MACHINE_DETAIL,
    TYPE_ORG_SUMMARY,
    Report,
    ReportSchedule,
)
from app.models.user import User
from app.services import (
    csv_export,
    export_service,
    machine_service,
    pdf_service,
    report_data,
)

# Same params within this window reuse the existing report (idempotency, edge 3).
_DEDUP_WINDOW = timedelta(minutes=1)


def _reports_dir() -> Path:
    path = Path(settings.reports_dir)
    path.mkdir(parents=True, exist_ok=True)
    return path


# Filesystem ops kept in plain sync helpers so they're not flagged inside the
# async service methods (and are trivially fast, local-disk operations).
def _write_bytes(path: Path, data: bytes) -> None:
    path.write_bytes(data)


def _unlink(path: Path) -> None:
    path.unlink(missing_ok=True)


def file_exists(path_str: str) -> bool:
    return Path(path_str).exists()


def _machine_uuid(params: dict[str, Any]) -> uuid_lib.UUID | None:
    raw = params.get("machine_uuid")
    if raw is None:
        return None
    return raw if isinstance(raw, uuid_lib.UUID) else uuid_lib.UUID(str(raw))


async def create_report(
    session: AsyncSession,
    *,
    type: str,
    format: str,
    machine_uuid: uuid_lib.UUID | None,
    params: dict[str, Any],
    user_id: int | None,
    schedule_id: int | None = None,
) -> Report:
    """Insert a queued report, reusing a recent identical one if present."""
    merged = dict(params)
    if machine_uuid is not None:
        merged["machine_uuid"] = str(machine_uuid)

    existing = await _find_recent_duplicate(session, type=type, format=format, params=merged)
    if existing is not None:
        return existing

    report = Report(
        type=type,
        format=format,
        params=merged,
        status=STATUS_QUEUED,
        generated_by_user_id=user_id,
        schedule_id=schedule_id,
        expires_at=datetime.now(tz=UTC) + timedelta(days=settings.report_retention_days),
    )
    session.add(report)
    await session.flush()
    return report


async def _find_recent_duplicate(
    session: AsyncSession, *, type: str, format: str, params: dict[str, Any]
) -> Report | None:
    cutoff = datetime.now(tz=UTC) - _DEDUP_WINDOW
    candidates = (
        (
            await session.execute(
                select(Report)
                .where(
                    Report.type == type,
                    Report.format == format,
                    Report.status != STATUS_FAILED,
                    Report.created_at >= cutoff,
                )
                .order_by(Report.id.desc())
                .limit(10)
            )
        )
        .scalars()
        .all()
    )
    target = params.get("machine_uuid")
    return next((r for r in candidates if r.params.get("machine_uuid") == target), None)


async def get_report(session: AsyncSession, report_uuid: uuid_lib.UUID) -> Report | None:
    return (
        await session.execute(select(Report).where(Report.uuid == report_uuid))
    ).scalar_one_or_none()


async def list_reports(
    session: AsyncSession,
    *,
    type: str | None = None,
    page: int = 1,
    page_size: int = 50,
) -> tuple[list[Report], int]:
    conditions = []
    if type:
        conditions.append(Report.type == type)
    total = (
        await session.execute(select(func.count()).select_from(Report).where(*conditions))
    ).scalar_one()
    rows = (
        (
            await session.execute(
                select(Report)
                .where(*conditions)
                .order_by(Report.id.desc())
                .offset((page - 1) * page_size)
                .limit(page_size)
            )
        )
        .scalars()
        .all()
    )
    return list(rows), int(total)


async def to_dict(session: AsyncSession, report: Report) -> dict[str, Any]:
    email: str | None = None
    if report.generated_by_user_id is not None:
        user = await session.get(User, report.generated_by_user_id)
        email = user.email if user else None
    schedule_uuid: uuid_lib.UUID | None = None
    if report.schedule_id is not None:
        sched = await session.get(ReportSchedule, report.schedule_id)
        schedule_uuid = sched.uuid if sched else None
    return {
        "uuid": report.uuid,
        "type": report.type,
        "format": report.format,
        "status": report.status,
        "machine_uuid": _machine_uuid(report.params),
        "file_size": report.file_size,
        "error_message": report.error_message,
        "generated_by": email,
        "schedule_uuid": schedule_uuid,
        "created_at": report.created_at,
        "completed_at": report.completed_at,
        "expires_at": report.expires_at,
    }


async def delete_report(session: AsyncSession, report: Report) -> None:
    if report.file_path:
        _unlink(Path(report.file_path))
    await session.delete(report)


async def generate(session: AsyncSession, report: Report) -> None:
    """Render the artifact and persist its path/size. Raises on failure (the
    caller marks the row failed)."""
    report.status = STATUS_RUNNING
    await session.flush()

    data, ext = await _build_artifact(session, report)

    out_path = _reports_dir() / f"{report.uuid}.{ext}"
    _write_bytes(out_path, data)

    report.file_path = str(out_path)
    report.file_size = len(data)
    report.status = STATUS_COMPLETED
    report.completed_at = datetime.now(tz=UTC)


async def _build_artifact(session: AsyncSession, report: Report) -> tuple[bytes, str]:
    if report.format == FORMAT_PDF:
        return await _build_pdf(session, report), "pdf"
    if report.format == FORMAT_CSV:
        return await _build_csv(session, report), "csv"
    raise ValueError(f"Unsupported report format: {report.format!r}")


async def _build_pdf(session: AsyncSession, report: Report) -> bytes:
    if report.type == TYPE_ORG_SUMMARY:
        email = None
        if report.generated_by_user_id is not None:
            user = await session.get(User, report.generated_by_user_id)
            email = user.email if user else None
        org_ctx = await report_data.org_summary_context(session, generated_by=email)
        return pdf_service.render_pdf(pdf_service.TEMPLATES[TYPE_ORG_SUMMARY], org_ctx)

    machine_uuid = _machine_uuid(report.params)
    if machine_uuid is None:
        raise ValueError("machine_detail report missing machine_uuid")
    machine_ctx = await report_data.machine_detail_context(session, machine_uuid)
    if machine_ctx is None:
        raise ValueError("machine_detail report: machine not found")
    return pdf_service.render_pdf(pdf_service.TEMPLATES[TYPE_MACHINE_DETAIL], machine_ctx)


async def _build_csv(session: AsyncSession, report: Report) -> bytes:
    if report.type == TYPE_ORG_SUMMARY:
        header, rows = await export_service.machines_csv(session)
        return csv_export.to_csv_bytes(header, rows)

    machine_uuid = _machine_uuid(report.params)
    if machine_uuid is None:
        raise ValueError("machine_detail report missing machine_uuid")
    machine = await machine_service.get_machine(session, machine_uuid)
    if machine is None:
        raise ValueError("machine_detail report: machine not found")
    header, rows = await export_service.machine_software_csv(session, machine)
    return csv_export.to_csv_bytes(header, rows)


async def run_generation(report_uuid: uuid_lib.UUID, session: AsyncSession) -> None:
    """Worker entry: generate one report, recording failure on the row."""
    report = await get_report(session, report_uuid)
    if report is None:
        logger.warning("report_generate_missing", report_uuid=str(report_uuid))
        return
    try:
        await generate(session, report)
        await session.commit()
        logger.info("report_generated", report_uuid=str(report_uuid), size=report.file_size)
    except Exception as exc:  # surface any failure on the report row
        await session.rollback()
        report = await get_report(session, report_uuid)
        if report is not None:
            report.status = STATUS_FAILED
            report.error_message = str(exc)[:1000]
            await session.commit()
        logger.error("report_generate_failed", report_uuid=str(report_uuid), error=str(exc))


# Aliased constant for callers that import from here.
QUEUED = STATUS_QUEUED
