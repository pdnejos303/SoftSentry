"""arq task: daily license compliance check (Module 6.4).

Reconciles license_expiring / license_expired / license_overused alerts against
current install counts + expiry dates. Idempotent — safe to run on a cron and
on demand. Opens its own DB session, independent of the request lifecycle.
"""

from __future__ import annotations

from typing import Any

from app.core.db import SessionLocal
from app.core.logging import logger
from app.services import license_service


async def license_compliance_check(ctx: dict[str, Any]) -> dict[str, int]:
    logger.info("license_compliance_start")
    async with SessionLocal() as session:
        result = await license_service.reconcile_alerts(session)
        await session.commit()
    logger.info("license_compliance_done", **result)
    return result
