"""Redis client singleton สำหรับแชร์ connection pool ทั่วทั้ง process."""

from __future__ import annotations

from redis.asyncio import Redis, from_url

from app.core.config import settings

# ใช้ module-level variable (ไม่ใช่ @lru_cache) เพราะ Redis object คือ connection pool
# ที่ต้องแชร์ข้าม request ทั้งหมดใน process เดียวกัน
# decode_responses=True → ค่าที่ได้กลับมาเป็น str เสมอ ไม่ต้อง .decode() ที่ caller
_redis: Redis | None = None


def get_redis() -> Redis:
    global _redis
    if _redis is None:
        _redis = from_url(settings.redis_url, decode_responses=True)  # type: ignore[no-untyped-call]
    return _redis
