"""Pydantic schemas for agent-facing endpoints."""

from __future__ import annotations

import uuid as uuid_lib
from datetime import datetime
from typing import Literal

from pydantic import BaseModel, ConfigDict, Field


class EnrollmentTokenCreateRequest(BaseModel):
    label: str | None = Field(default=None, max_length=255)
    expires_in_seconds: int = Field(default=24 * 3600, ge=60, le=7 * 24 * 3600)


class EnrollmentTokenCreated(BaseModel):
    uuid: uuid_lib.UUID
    token: str = Field(..., description="plaintext — show once, never stored")
    expires_at: datetime
    label: str | None


class AgentEnrollRequest(BaseModel):
    enrollment_token: str
    hostname: str = Field(..., max_length=255)
    os: Literal["windows", "macos", "linux"]
    os_version: str = Field(..., max_length=50)
    arch: Literal["amd64", "arm64", "x86"]
    agent_version: str = Field(..., max_length=20)
    # Stable hardware id (Windows MachineGuid / macOS IOPlatformUUID). Optional so
    # older agents still enroll; when present the server dedups re-enrollment onto
    # the existing machine instead of creating a duplicate.
    fingerprint: str | None = Field(default=None, max_length=255)


class AgentEnrollResponse(BaseModel):
    machine_uuid: uuid_lib.UUID
    agent_token: str = Field(..., description="permanent — agent stores in token file")
    scan_interval_hours: int = 6


class MachineOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    uuid: uuid_lib.UUID
    hostname: str
    os: str
    os_version: str
    arch: str
    agent_version: str
    status: str
    enrolled_at: datetime
    last_seen_at: datetime | None


class AgentConfigOut(BaseModel):
    """Config the agent pulls (protocol §4) — drives scan cadence + auto-update."""

    scan_interval_hours: int = 6
    auto_update_enabled: bool = True
    manual_scan_requested: bool = False


class AgentUpdateAvailable(BaseModel):
    version: str
    download_url: str
    sha256: str


class HeartbeatRequest(BaseModel):
    agent_version: str | None = Field(default=None, max_length=20)
    uptime_seconds: int | None = Field(default=None, ge=0)


class HeartbeatResponse(BaseModel):
    """Signals the agent acts on after each liveness ping (protocol §2)."""

    config_changed: bool = False
    manual_scan_requested: bool = False
    agent_update_available: AgentUpdateAvailable | None = None
