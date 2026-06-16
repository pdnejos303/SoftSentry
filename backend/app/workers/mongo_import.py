"""arq job — periodic MongoDB → PostgreSQL inventory import (Module 11)."""

from __future__ import annotations

from typing import Any

from app.services import mongo_import_service


async def mongo_import(
    ctx: dict[str, Any], *, lookback_minutes: int | None = None
) -> dict[str, int]:
    result = await mongo_import_service.run_import(lookback_minutes=lookback_minutes)
    return {
        "documents_seen": result.documents_seen,
        "machines_imported": result.machines_imported,
        "errors": result.errors,
    }

async def enqueue_mongo_import(*, lookback_minutes: int | None = None) -> bool:
    """Enqueue a `mongo_import` job on the arq queue. Returns False if the queue
    is unreachable (caller decides how to surface it)."""
    from arq import create_pool

    from app.workers import WorkerSettings

    try:
        pool = await create_pool(WorkerSettings.redis_settings)
    except (OSError, ConnectionError):
        return False
    try:
        await pool.enqueue_job("mongo_import", lookback_minutes=lookback_minutes)
    finally:
        await pool.aclose()
    return True