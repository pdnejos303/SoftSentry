"""Application settings โหลดจาก environment ผ่าน pydantic-settings."""

from __future__ import annotations

from functools import lru_cache
from typing import Literal

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
        extra="ignore",
    )

    env: Literal["development", "staging", "production"] = "development"
    log_level: str = "INFO"
    enable_docs: bool = True

    database_url: str
    redis_url: str = "redis://localhost:6379/0"

    jwt_secret: str
    jwt_access_ttl_seconds: int = 3600
    jwt_refresh_ttl_seconds: int = 60 * 60 * 24 * 30
    jwt_algorithm: str = "HS256"
    bcrypt_pepper: str = ""

    license_key_encryption_key: str = Field(
        default="",
        description="hex32 key สำหรับ Fernet — สร้างด้วย openssl rand -hex 32",
    )

    dashboard_url: str = "http://localhost:3000"

    initial_admin_email: str = "admin@local"
    initial_admin_password: str = "ChangeMe!2026"

    nvd_api_key: str = ""
    osv_api_base: str = "https://api.osv.dev/v1"

    # โฟลเดอร์ที่เก็บ agent binary + manifest.json สำหรับ auto-update (spec 1.6)
    # ถ้าว่างเปล่า จะปิด endpoint auto-update distribution
    agent_binary_dir: str = ""

    # โฟลเดอร์สำหรับ report artifact (PDF/CSV) ที่สร้างออกมา (spec 8.4)
    # mount เป็น volume ใน prod เพื่อให้ไฟล์รอดจาก container restart
    reports_dir: str = "var/reports"
    # report ที่เก่ากว่านี้จะถูก cleanup job รายวันลบทิ้ง
    report_retention_days: int = 30

    # ความถี่ที่ in-process loop รีเฟรช business-KPI gauge ที่ /metrics (Module: Telemetry)
    # ต้องรันใน process เดียวกับ backend เพราะ Prometheus gauge อยู่ใน memory ของ process
    # arq worker เป็น process แยกต่างหาก
    metrics_refresh_seconds: int = 30

    @property
    def is_production(self) -> bool:
        return self.env == "production"


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    return Settings()


settings = get_settings()
