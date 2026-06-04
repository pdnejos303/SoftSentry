"""Shared pytest fixtures.

Uses SQLite (aiosqlite) for tests so the suite is self-contained and CI doesn't
need Postgres. Schema is created via SQLAlchemy `create_all` — for production
SQL feature coverage (JSONB GIN, ILIKE-trigram) integration tests run against
Postgres in docker-compose.
"""

from __future__ import annotations

import os

# Set env BEFORE importing app modules so Settings picks them up.
os.environ.setdefault("DATABASE_URL", "sqlite+aiosqlite:///:memory:")
os.environ.setdefault("REDIS_URL", "redis://localhost:6379/15")
os.environ.setdefault("JWT_SECRET", "test_secret_change_me_for_real_runs")
os.environ.setdefault("ENV", "development")

from typing import TYPE_CHECKING

import pytest_asyncio
from app.core import db as db_module
from app.core import redis as redis_module
from app.main import app
from app.models import Base
from httpx import ASGITransport, AsyncClient
from sqlalchemy.ext.asyncio import async_sessionmaker, create_async_engine

if TYPE_CHECKING:
    from collections.abc import AsyncIterator


class FakeRedis:
    """In-memory Redis substitute — supports ops we use."""

    def __init__(self) -> None:
        self._store: dict[str, str] = {}
        self._expires: dict[str, float] = {}

    async def get(self, key: str) -> str | None:
        return self._store.get(key)

    async def set(self, key: str, value: str, ex: int | None = None) -> None:
        self._store[key] = value
        if ex:
            import time

            self._expires[key] = time.time() + ex

    async def delete(self, *keys: str) -> int:
        n = 0
        for k in keys:
            if k in self._store:
                del self._store[k]
                self._expires.pop(k, None)
                n += 1
        return n

    async def incr(self, key: str) -> int:
        new = int(self._store.get(key, "0")) + 1
        self._store[key] = str(new)
        return new

    async def expire(self, key: str, seconds: int) -> bool:
        import time

        if key in self._store:
            self._expires[key] = time.time() + seconds
            return True
        return False

    async def ttl(self, key: str) -> int:
        import time

        exp = self._expires.get(key)
        if exp is None:
            return -1 if key in self._store else -2
        remaining = int(exp - time.time())
        return remaining if remaining > 0 else -2


@pytest_asyncio.fixture
async def fake_redis() -> AsyncIterator[FakeRedis]:
    r = FakeRedis()
    redis_module._redis = r  # type: ignore[assignment]
    yield r
    redis_module._redis = None


@pytest_asyncio.fixture
async def engine():
    eng = create_async_engine(os.environ["DATABASE_URL"], echo=False)
    async with eng.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    yield eng
    await eng.dispose()


@pytest_asyncio.fixture
async def session(engine):
    SessionTest = async_sessionmaker(bind=engine, expire_on_commit=False)
    db_module.SessionLocal = SessionTest  # type: ignore[assignment]
    async with SessionTest() as s:
        yield s


@pytest_asyncio.fixture
async def client(session, fake_redis) -> AsyncIterator[AsyncClient]:
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as c:
        yield c
