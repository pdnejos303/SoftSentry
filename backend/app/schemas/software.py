"""Pydantic schemas for inventory endpoints.

A3 — per-machine: software list, install/uninstall history, scan history.
A4 — cross-machine: aggregate software view, top-N, two-machine compare.
"""

from __future__ import annotations

import uuid as uuid_lib
from datetime import date, datetime

from pydantic import BaseModel, Field

# ── A3: per-machine software ────────────────────────────────────────────────


class MachineSoftwareItem(BaseModel):
    uuid: uuid_lib.UUID
    name: str
    version: str
    publisher: str | None = None
    install_date: date | None = None
    install_path: str | None = None
    install_size_kb: int | None = None
    arch: str | None = None
    source: str
    signature_status: str | None = None  # valid|expired|invalid|unsigned|None
    is_active: bool = True
    license_status: str | None = None  # compliant|over_used|expired|expiring_soon|None


class MachineSoftwareList(BaseModel):
    items: list[MachineSoftwareItem]
    total: int
    page: int
    page_size: int
    total_pages: int


# ── A3: history ─────────────────────────────────────────────────────────────


class SoftwareHistoryItem(BaseModel):
    software_name: str
    software_version: str
    event: str  # installed | uninstalled | updated
    previous_version: str | None = None
    occurred_at: datetime


class SoftwareHistoryList(BaseModel):
    items: list[SoftwareHistoryItem]
    total: int
    page: int
    page_size: int
    total_pages: int


# ── Recently-installed feed + trust (security posture) ──────────────────────


class RecentInstallItem(BaseModel):
    machine_uuid: uuid_lib.UUID
    machine_name: str
    name: str
    version: str
    event: str  # installed | updated
    publisher: str | None = None
    detected_at: datetime  # when the agent first saw it
    installed_at: datetime | None = None  # accurate install moment (best-effort)
    install_date: date | None = None  # registry date-only fallback
    signature_status: str | None = None  # valid|expired|invalid|unsigned|None
    trust: str  # trusted | suspicious | risky


class RecentInstallList(BaseModel):
    items: list[RecentInstallItem]
    total: int


# ── A3: scans ───────────────────────────────────────────────────────────────


class ScanHistoryItem(BaseModel):
    uuid: uuid_lib.UUID
    started_at: datetime
    completed_at: datetime
    received_at: datetime
    software_count: int
    scan_type: str
    trigger: str | None = None


class ScanHistoryList(BaseModel):
    items: list[ScanHistoryItem]
    total: int
    page: int            
    page_size: int     
    total_pages: int    


# ── A4: cross-machine software ──────────────────────────────────────────────


class MachineRef(BaseModel):
    """A machine an app is installed on, with a readable label for the
    cross-machine drill-down (no extra round-trip to resolve uuids)."""

    uuid: uuid_lib.UUID
    name: str


class CrossSoftwareItem(BaseModel):
    name: str
    version: str
    publisher: str | None = None
    installed_count: int
    machines: list[MachineRef] = Field(default_factory=list)  # capped at 10
    signature_status: str | None = None


class CrossSoftwareList(BaseModel):
    items: list[CrossSoftwareItem]
    total: int
    page: int
    page_size: int
    total_pages: int


class SoftwareStats(BaseModel):
    """Fleet-wide posture summary for the inventory header strip."""

    unique_apps: int
    total_installs: int
    valid: int
    unsigned: int
    invalid: int


class TopSoftwareItem(BaseModel):
    name: str
    publisher: str | None = None
    installed_count: int


# ── A4: compare ─────────────────────────────────────────────────────────────


class CompareRequest(BaseModel):
    machine_uuids: list[uuid_lib.UUID] = Field(..., min_length=2, max_length=2)


class CompareEntry(BaseModel):
    name: str
    version: str


class VersionDiffEntry(BaseModel):
    name: str
    version_a: str
    version_b: str


class CompareResponse(BaseModel):
    common: list[CompareEntry]
    only_in_a: list[CompareEntry]
    only_in_b: list[CompareEntry]
    version_diff: list[VersionDiffEntry]

# ── for each software detail page ─────────────────────────────────────────────────────────────
class SoftwareVersionCount(BaseModel):
    version: str
    installed_count: int


class SoftwareDetailOut(BaseModel):
    name: str
    publisher: str | None = None
    installed_count: int                      # distinct machines 
    versions: list[SoftwareVersionCount]      # Keep a list of all versions of this software, along with the number of machines that have each version installed.
    signature_status: str | None = None       
    license_status: str | None = None


class SoftwareMachineItem(BaseModel):
    machine_uuid: uuid_lib.UUID
    hostname: str
    display_name: str | None = None
    owner: str | None = None
    status: str                               # online | stale | offline
    version: str
    install_date: date | None = None
    signature_status: str | None = None

class SoftwareMachineList(BaseModel):
    items: list[SoftwareMachineItem]
    total: int
    page: int
    page_size: int
    total_pages: int