"""Pydantic schemas for the one-click deployment endpoints (admin)."""

from __future__ import annotations

import uuid as uuid_lib
from datetime import datetime

from pydantic import BaseModel, ConfigDict, Field


class DeploymentTokenCreateRequest(BaseModel):
    label: str | None = Field(default=None, max_length=255)
    # None → never expires. Bounded to a year to avoid forgotten eternal tokens.
    expires_in_days: int | None = Field(default=None, ge=1, le=365)
    # None → unlimited machines may enrol with this one link.
    max_uses: int | None = Field(default=None, ge=1, le=100_000)


class DeploymentTokenCreated(BaseModel):
    uuid: uuid_lib.UUID
    token: str = Field(..., description="plaintext — show once, used to build the install link")
    label: str | None
    expires_at: datetime
    max_uses: int | None


class DeploymentTokenOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    uuid: uuid_lib.UUID
    label: str | None
    expires_at: datetime
    max_uses: int | None
    use_count: int
    revoked_at: datetime | None
    created_at: datetime


class BinaryInfoOut(BaseModel):
    """The agent binary currently being served — shown as a version badge on the
    deploy page so an admin can confirm which build a downloaded installer carries."""

    version: str
    os: str
    arch: str
    # sha256 ของ binary ที่เสิร์ฟอยู่ — เปลี่ยนทุกครั้งที่ build ใหม่ (ต่างจาก version
    # ที่ถูก fix เป็น 0.1.0 เสมอ) ใช้เป็น "build fingerprint" ให้ admin ยืนยันบนหน้า
    # deploy ได้ว่าตัวที่กำลังเสิร์ฟ/กำลังจะโหลด เป็นบิลด์ใหม่จริง ไม่ใช่ของเก่าค้าง
    sha256: str
    # build_stamp = UTC timestamp ของตอน build ซึ่งถูก "bake เข้า binary" ด้วย → installer
    # wizard โชว์ค่าเดียวกันนี้บนหน้าแรก ผู้ใช้จึงเทียบ "ค่าบนหน้า deploy" กับ "ค่าบนตัวที่
    # โหลดไปรัน" ได้ตรงๆ ตรงกัน = ตัวที่โหลด = build ล่าสุดแน่นอน ("" = manifest เก่า)
    build_stamp: str = ""
