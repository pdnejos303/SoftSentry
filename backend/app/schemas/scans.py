"""Pydantic schemas for scan ingestion (POST /api/v1/agents/scans)."""

from __future__ import annotations

import uuid as uuid_lib
from datetime import date, datetime
from typing import Literal

from pydantic import BaseModel, Field

SignatureStatus = Literal["valid", "expired", "invalid", "unsigned", "unknown"]


class SignatureIn(BaseModel):
    status: SignatureStatus
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
    install_path: str | None = None
    install_size_kb: int | None = Field(default=None, ge=0)
    arch: str | None = Field(default=None, max_length=10)
    source: Literal["registry", "appstore", "plist", "filesystem"]
    signature: SignatureIn | None = None


class ScanIn(BaseModel):
    started_at: datetime
    completed_at: datetime
    scan_type: Literal["auto", "manual"] = "auto"
    trigger: str | None = Field(default=None, max_length=20)
    software: list[SoftwareIn] = Field(default_factory=list)


class ScanAccepted(BaseModel):
    scan_uuid: uuid_lib.UUID
    queued_for_analysis: bool = True
