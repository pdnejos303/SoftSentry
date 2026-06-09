"""arq worker — background jobs (CVE sync, report generation ฯลฯ)

ลงทะเบียน task `cve_sync` (Module 5.1) แบบ daily cron + task `ping` เป็น no-op
เพื่อให้ worker boot ได้แม้ทุก job อื่นจะถูก disable
"""

from __future__ import annotations

from typing import Any, ClassVar

from arq import cron
from arq.connections import RedisSettings

from app.core.config import settings
from app.workers.cve_sync import cve_sync
from app.workers.license_compliance import license_compliance_check
from app.workers.report_jobs import cleanup_reports, generate_report, run_due_schedules


def _redis_settings() -> RedisSettings:
    return RedisSettings.from_dsn(settings.redis_url)


async def ping(ctx: dict[str, Any]) -> str:
    """No-op placeholder task — ให้ arq start worker ได้ก่อนที่ job จริงจะพร้อม."""
    return "pong"


class WorkerSettings:
    functions: ClassVar[list[Any]] = [
        ping,
        cve_sync,
        license_compliance_check,
        generate_report,
        run_due_schedules,
        cleanup_reports,
    ]
    cron_jobs: ClassVar[list[Any]] = [
        # sync CVE รายวัน 03:00 UTC (spec 5.1)
        cron(cve_sync, hour=3, minute=0),
        # ตรวจ license compliance + expiry รายวัน 04:00 UTC (spec 6.4)
        cron(license_compliance_check, hour=4, minute=0),
        # ยิง scheduled report ที่ถึงเวลาทุกนาที (spec 8.5)
        cron(run_due_schedules, second=0),
        # ลบ report artifact ที่หมดอายุ รายวัน 05:00 UTC (spec 8.4)
        cron(cleanup_reports, hour=5, minute=0),
    ]
    redis_settings = _redis_settings()
    max_jobs = 10
