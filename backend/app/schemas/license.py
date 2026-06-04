"""Pydantic schemas for license compliance (Module 6)."""

from __future__ import annotations

import uuid as uuid_lib
from datetime import date, datetime
from typing import Literal

from pydantic import BaseModel, Field

LicenseStatus = Literal["compliant", "over_used", "expired", "expiring_soon"]


class LicenseBase(BaseModel):
    software_name: str = Field(..., min_length=1, max_length=500)
    publisher: str | None = Field(default=None, max_length=255)
    purchased_count: int = Field(..., ge=1)
    purchased_at: date | None = None
    expires_at: date | None = None
    cost_total: float | None = Field(default=None, ge=0)
    currency: str = Field(default="THB", min_length=3, max_length=3)
    vendor_contact: str | None = Field(default=None, max_length=255)
    notes: str | None = None


class LicenseIn(LicenseBase):
    license_key: str | None = Field(default=None, max_length=2000)


class LicenseUpdate(BaseModel):
    software_name: str | None = Field(default=None, min_length=1, max_length=500)
    publisher: str | None = Field(default=None, max_length=255)
    license_key: str | None = Field(default=None, max_length=2000)
    purchased_count: int | None = Field(default=None, ge=1)
    purchased_at: date | None = None
    expires_at: date | None = None
    cost_total: float | None = Field(default=None, ge=0)
    currency: str | None = Field(default=None, min_length=3, max_length=3)
    vendor_contact: str | None = Field(default=None, max_length=255)
    notes: str | None = None


class LicenseOut(LicenseBase):
    uuid: uuid_lib.UUID
    has_license_key: bool
    installed_count: int
    status: LicenseStatus
    days_until_expiry: int | None
    created_at: datetime
    updated_at: datetime


class LicenseDetail(LicenseOut):
    # Decrypted key — returned only on the single-license GET (admin form needs it).
    license_key: str | None = None


class LicenseList(BaseModel):
    items: list[LicenseOut]
    total: int
    page: int
    page_size: int
    total_pages: int


class LicenseInstallation(BaseModel):
    machine_uuid: uuid_lib.UUID
    hostname: str
    os: str
    software_name: str
    software_version: str
    last_seen_at: datetime | None


class LicenseInstallationList(BaseModel):
    license_uuid: uuid_lib.UUID
    software_name: str
    items: list[LicenseInstallation]
    total: int


class ComplianceSummary(BaseModel):
    total: int
    compliant: int
    over_used: int
    expired: int
    expiring_30: int
    expiring_60: int
    expiring_90: int
    compliance_rate: int  # percentage 0-100


class RefreshResult(BaseModel):
    licenses: int
    alerts_raised: int
    alerts_resolved: int
