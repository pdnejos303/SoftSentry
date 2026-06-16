from __future__ import annotations

from functools import lru_cache
from typing import TYPE_CHECKING

from app.core.config import settings

if TYPE_CHECKING:
    from motor.motor_asyncio import AsyncIOMotorClient, AsyncIOMotorCollection


@lru_cache(maxsize=1)
def get_mongo_client() -> AsyncIOMotorClient:
    """Lazily create the shared motor client. Call only when mongo is enabled."""
    if not settings.mongo_import_enabled:
        raise RuntimeError("MONGO_URI is not configured; mongo import is disabled")
    # Imported here so the dependency is optional at startup if the feature is off.
    from motor.motor_asyncio import AsyncIOMotorClient

    return AsyncIOMotorClient(settings.mongo_uri, tz_aware=True)


def get_source_collection() -> AsyncIOMotorCollection:
    """The collection holding inventory documents (config-driven)."""
    client = get_mongo_client()
    return client[settings.mongo_db][settings.mongo_collection]


async def ping() -> bool:
    """Health check — returns True if the server answers."""
    client = get_mongo_client()
    await client.admin.command("ping")
    return True