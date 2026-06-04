"""Async Redis client singleton."""

from __future__ import annotations

from redis.asyncio import Redis, from_url

from app.core.config import settings

_redis: Redis | None = None


def get_redis() -> Redis:
    global _redis
    if _redis is None:
        _redis = from_url(settings.redis_url, decode_responses=True)  # type: ignore[no-untyped-call]
    return _redis
