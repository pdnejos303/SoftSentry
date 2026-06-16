"""arq worker — background jobs (CVE sync, report generation, etc.).

Registers the `cve_sync` task (Module 5.1) with a daily cron, plus a no-op
`ping` so the worker boots even if everything else is disabled.
"""

from __future__ import annotations

from typing import Any, ClassVar

from arq import cron
from arq.connections import RedisSettings

from app.core.config import settings
from app.workers.cve_sync import cve_sync
from app.workers.license_compliance import license_compliance_check
from app.workers.report_jobs import cleanup_reports, generate_report, run_due_schedules
from app.workers.mongo_import import mongo_import 


def _redis_settings() -> RedisSettings:
    return RedisSettings.from_dsn(settings.redis_url)


async def ping(ctx: dict[str, Any]) -> str:
    """No-op placeholder task. Lets arq start a worker before real jobs exist."""
    return "pong"


class WorkerSettings:
    functions: ClassVar[list[Any]] = [
        ping,
        cve_sync,
        license_compliance_check,
        generate_report,
        run_due_schedules,
        cleanup_reports,
        mongo_import,
    ]
    cron_jobs: ClassVar[list[Any]] = [
        # Daily incremental CVE sync at 03:00 UTC (spec 5.1).
        cron(cve_sync, hour=3, minute=0),
        # Daily license compliance / expiry check at 04:00 UTC (spec 6.4).
        cron(license_compliance_check, hour=4, minute=0),
        # Fire due report schedules every minute (spec 8.5).
        cron(run_due_schedules, second=0),
        # Purge expired report artifacts daily at 05:00 UTC (spec 8.4).
        cron(cleanup_reports, hour=5, minute=0),
        # Check and retreive data from mongoDB every 30 minutes
        cron(mongo_import, minute={0, 30}),
    ]
    redis_settings = _redis_settings()
    max_jobs = 10
