"""Async SQLAlchemy engine, session factory และ FastAPI dependency."""

from __future__ import annotations

from typing import TYPE_CHECKING, Annotated

from fastapi import Depends
from sqlalchemy.ext.asyncio import (
    AsyncSession,
    async_sessionmaker,
    create_async_engine,
)

from app.core.config import settings

if TYPE_CHECKING:
    from collections.abc import AsyncIterator

_engine_kwargs: dict[str, object] = {
    "echo": settings.log_level == "DEBUG",
}
# ปรับ connection pool สำหรับ Postgres — SQLite ที่ใช้ใน test ไม่รับ pool_size/max_overflow
# และจัดการ connection ด้วย pool ของตัวเอง
if not settings.database_url.startswith("sqlite"):
    _engine_kwargs.update(pool_pre_ping=True, pool_size=10, max_overflow=20)

engine = create_async_engine(settings.database_url, **_engine_kwargs)

SessionLocal = async_sessionmaker(
    bind=engine,
    expire_on_commit=False,
    class_=AsyncSession,
)


async def get_session() -> AsyncIterator[AsyncSession]:
    async with SessionLocal() as session:
        try:
            yield session
        except Exception:
            await session.rollback()
            raise


DBSession = Annotated[AsyncSession, Depends(get_session)]
