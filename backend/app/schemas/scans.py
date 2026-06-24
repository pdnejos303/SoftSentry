"""Pydantic schemas for scan ingestion (POST /api/v1/agents/scans)."""

from __future__ import annotations

import uuid as uuid_lib
from datetime import date, datetime
from typing import Literal

from pydantic import BaseModel, Field

SignatureStatus = Literal["valid", "expired", "invalid", "unsigned", "unknown"]


class SignatureIn(BaseModel):
    status: SignatureStatus
    # Why verification failed (only for status="invalid"): a reason code
    # (tampered / untrusted_root / broken_chain / revoked / distrusted) or a raw
    # HRESULT hex (e.g. "0x800B0004") the agent couldn't map. Optional for
    # back-compat with older agents.
    status_reason: str | None = Field(default=None, max_length=40)
    signer: str | None = Field(default=None, max_length=500)
    issuer: str | None = Field(default=None, max_length=500)
    cert_thumbprint: str | None = Field(default=None, max_length=64)
    cert_valid_from: date | None = None
    cert_valid_to: date | None = None
    signature_algorithm: str | None = Field(default=None, max_length=50)
    chain: list[dict[str, object]] | None = None


class SoftwareIn(BaseModel):
    name: str = Field(..., min_length=1, max_length=500)
    version: str = Field(..., max_length=100)
    publisher: str | None = Field(default=None, max_length=255)
    install_date: date | None = None
    # Accurate install timestamp (best-effort, e.g. install-dir creation time).
    # Optional for back-compat: older agents send only install_date.
    installed_at: datetime | None = None
    install_path: str | None = None
    install_size_kb: int | None = Field(default=None, ge=0)
    arch: str | None = Field(default=None, max_length=10)
    source: Literal["registry", "appstore", "plist", "filesystem"]
    signature: SignatureIn | None = None


class DeviceSystemIn(BaseModel):
    manufacturer: str | None = None
    model: str | None = None
    serial_number: str | None = None
    system_type: str | None = None
    domain: str | None = None
    total_ram_mb: int | None = None


class DeviceCpuIn(BaseModel):
    model: str | None = None
    manufacturer: str | None = None
    cores: int | None = None
    logical_count: int | None = None
    clock_mhz: int | None = None
    architecture: str | None = None


class DeviceMemoryModuleIn(BaseModel):
    capacity_mb: int | None = None
    speed_mhz: int | None = None
    manufacturer: str | None = None
    part_number: str | None = None
    slot: str | None = None


class DeviceMemoryIn(BaseModel):
    total_mb: int | None = None
    modules: list[DeviceMemoryModuleIn] = Field(default_factory=list)


class DeviceDiskIn(BaseModel):
    model: str | None = None
    size_gb: int | None = None
    media_type: str | None = None
    interface_type: str | None = None
    serial: str | None = None


class DeviceGpuIn(BaseModel):
    name: str | None = None
    driver_version: str | None = None
    vram_mb: int | None = None


class DeviceNicIn(BaseModel):
    name: str | None = None
    mac: str | None = None
    type: str | None = None


class DeviceFirmwareIn(BaseModel):
    bios_vendor: str | None = None
    bios_version: str | None = None
    bios_date: str | None = None
    motherboard: str | None = None
    board_serial: str | None = None


class DeviceSecurityIn(BaseModel):
    secure_boot: str | None = None
    tpm_present: bool = False
    tpm_enabled: bool = False
    tpm_version: str | None = None


class DeviceBatteryIn(BaseModel):
    name: str | None = None
    charge_percent: int | None = None
    status: str | None = None


class DeviceMonitorIn(BaseModel):
    name: str | None = None
    width: int | None = None
    height: int | None = None


class DevicePendingUpdateIn(BaseModel):
    kb: str | None = None
    title: str | None = None
    security: bool = False
    severity: str | None = None


class DeviceWindowsUpdateIn(BaseModel):
    status: str = "unknown"
    pending_count: int = 0
    security_pending: int = 0
    reboot_pending: bool = False
    last_installed_kb: str | None = None
    last_installed_at: str | None = None
    last_checked_at: datetime | None = None
    source: str | None = None
    pending: list[DevicePendingUpdateIn] = Field(default_factory=list)


class DeviceIn(BaseModel):
    """Hardware inventory + Windows Update posture (optional on a scan).

    Older agents — and agents on platforms that can't collect it (macOS this
    round) — omit this entirely, so every field is optional and the scan still
    ingests normally.
    """

    system: DeviceSystemIn = Field(default_factory=DeviceSystemIn)
    cpu: DeviceCpuIn = Field(default_factory=DeviceCpuIn)
    memory: DeviceMemoryIn = Field(default_factory=DeviceMemoryIn)
    disks: list[DeviceDiskIn] = Field(default_factory=list)
    gpus: list[DeviceGpuIn] = Field(default_factory=list)
    network: list[DeviceNicIn] = Field(default_factory=list)
    firmware: DeviceFirmwareIn = Field(default_factory=DeviceFirmwareIn)
    security: DeviceSecurityIn = Field(default_factory=DeviceSecurityIn)
    battery: DeviceBatteryIn | None = None
    monitors: list[DeviceMonitorIn] = Field(default_factory=list)
    windows_update: DeviceWindowsUpdateIn | None = None


class ScanIn(BaseModel):
    started_at: datetime
    completed_at: datetime
    scan_type: Literal["auto", "manual"] = "auto"
    trigger: str | None = Field(default=None, max_length=20)
    software: list[SoftwareIn] = Field(default_factory=list)
    # Hardware + Windows Update posture (Phase 6). Optional for back-compat.
    device: DeviceIn | None = None


class ScanAccepted(BaseModel):
    scan_uuid: uuid_lib.UUID
    queued_for_analysis: bool = True
