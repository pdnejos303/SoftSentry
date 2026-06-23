"""Pydantic schemas for dashboard machine endpoints."""

from __future__ import annotations

import uuid as uuid_lib
from datetime import datetime

from pydantic import BaseModel, Field


class VulnerabilityCount(BaseModel):
    """Per-severity vuln tally. Populated in Phase 3 (CVE matching); zeros for now."""

    critical: int = 0
    high: int = 0
    medium: int = 0
    low: int = 0


class ScanProgress(BaseModel):
    """Live scan progress for the dashboard bar. Present only while a scan runs
    (phase != idle); ``None`` on the machine means no bar is shown."""

    phase: str
    done: int = 0
    total: int = 0
    current_path: str | None = None
    updated_at: datetime | None = None


class MachineListItem(BaseModel):
    uuid: uuid_lib.UUID
    hostname: str
    display_name: str | None = None
    owner: str | None = None
    os: str
    os_version: str
    agent_version: str
    status: str
    # Backend URL the agent reports phoning home to (None until first heartbeat
    # from an agent new enough to send it).
    reported_server_url: str | None = None
    last_seen_at: datetime | None
    last_scan_at: datetime | None
    tags: list[str]
    software_count: int
    vulnerability_count: VulnerabilityCount = Field(default_factory=VulnerabilityCount)
    risk_score: float = 0
    # Live scan progress (None when idle / never scanned) so the list and detail
    # views can render a progress bar that updates as heartbeats arrive.
    scan_progress: ScanProgress | None = None


class MachineDetail(MachineListItem):
    arch: str
    enrolled_at: datetime


class MachineList(BaseModel):
    items: list[MachineListItem]
    total: int
    page: int
    page_size: int
    total_pages: int


class MachineUpdate(BaseModel):
    """Admin edit. All fields optional — only those present in the request body
    are applied (partial update), so renaming a machine doesn't clobber its tags
    and vice versa. ``None`` is a valid value to clear display_name/owner."""

    tags: list[str] | None = Field(default=None, max_length=50)
    display_name: str | None = Field(default=None, max_length=255)
    owner: str | None = Field(default=None, max_length=255)


class TriggerScanResponse(BaseModel):
    manual_scan_requested: bool = True


class TriggerUpdateResponse(BaseModel):
    force_update_requested: bool = True
