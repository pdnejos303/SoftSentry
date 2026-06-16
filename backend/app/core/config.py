"""Application settings loaded from environment via pydantic-settings."""

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
        description="hex32 key for Fernet — generate with openssl rand -hex 32",
    )

    dashboard_url: str = "http://localhost:3000"

    initial_admin_email: str = "admin@local"
    initial_admin_password: str = "ChangeMe!2026"

    nvd_api_key: str = ""
    osv_api_base: str = "https://api.osv.dev/v1"

    # ─── MongoDB ingestion (Module 11) ───────────────────────────
    # Blank MONGO_URI disables the importer (worker cron + endpoint no-op).
    mongo_uri: str = ""
    mongo_db: str = "softsentry"
    mongo_collection: str = "inventory"
    mongo_import_lookback_minutes: int = 1440
    mongo_timestamp_field: str = "scanned_at"

    @property
    def mongo_import_enabled(self) -> bool:
        return bool(self.mongo_uri)

    # Directory holding agent binaries + manifest.json for auto-update
    # (spec 1.6). Empty disables the auto-update distribution endpoints.
    agent_binary_dir: str = ""

    # Where generated report artifacts (PDF/CSV) are written (spec 8.4). Mounted
    # as a volume in prod so files survive container restarts.
    reports_dir: str = "var/reports"
    # Generated reports older than this are purged by the daily cleanup job.
    report_retention_days: int = 30

    # How often the in-process loop refreshes business-KPI gauges exposed at
    # /metrics (Module: Telemetry). Kept in-process because Prometheus gauges
    # live in the backend's memory; the arq worker is a separate process.
    metrics_refresh_seconds: int = 30

    @property
    def is_production(self) -> bool:
        return self.env == "production"


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    return Settings()


settings = get_settings()
