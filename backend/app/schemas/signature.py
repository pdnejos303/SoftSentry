"""Pydantic schemas for the signature endpoints (Module 3)."""

from __future__ import annotations

import uuid as uuid_lib
from datetime import date, datetime

from pydantic import BaseModel


class SignatureListItem(BaseModel):
    software_uuid: uuid_lib.UUID
    name: str
    version: str
    publisher: str | None = None
    machine_uuid: uuid_lib.UUID
    machine_hostname: str
    status: str
    status_reason: str | None = None
    signer: str | None = None
    cert_valid_to: date | None = None


class SignatureList(BaseModel):
    items: list[SignatureListItem]
    total: int
    page: int
    page_size: int
    total_pages: int


class SignatureDetail(BaseModel):
    software_uuid: uuid_lib.UUID
    software_name: str
    software_version: str
    machine_uuid: uuid_lib.UUID
    machine_hostname: str
    status: str
    status_reason: str | None = None
    signer: str | None = None
    issuer: str | None = None
    cert_thumbprint: str | None = None
    cert_valid_from: date | None = None
    cert_valid_to: date | None = None
    signature_algorithm: str | None = None
    chain: list[dict[str, object]] | None = None
    verified_at: datetime
    issues: list[str] = []


class SignatureStatsItem(BaseModel):
    status: str
    count: int


class SignatureStats(BaseModel):
    total: int
    distribution: list[SignatureStatsItem]
