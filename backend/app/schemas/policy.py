"""Pydantic schemas for whitelist/blacklist policy + bulk import (Module 4)."""

from __future__ import annotations

import uuid as uuid_lib
from datetime import datetime
from typing import Literal

from pydantic import BaseModel, ConfigDict, Field

Severity = Literal["high", "medium", "low"]


class PolicyEntryBase(BaseModel):
    name_pattern: str = Field(..., min_length=1, max_length=500)
    publisher_pattern: str | None = Field(default=None, max_length=255)
    version_constraint: str | None = Field(default=None, max_length=100)
    notes: str | None = None


# ── Whitelist ───────────────────────────────────────────────────────────────
class WhitelistEntryIn(PolicyEntryBase):
    pass


class WhitelistEntryUpdate(BaseModel):
    name_pattern: str | None = Field(default=None, min_length=1, max_length=500)
    publisher_pattern: str | None = Field(default=None, max_length=255)
    version_constraint: str | None = Field(default=None, max_length=100)
    notes: str | None = None


class WhitelistEntryOut(PolicyEntryBase):
    model_config = ConfigDict(from_attributes=True)
    uuid: uuid_lib.UUID
    created_at: datetime
    updated_at: datetime


class WhitelistList(BaseModel):
    items: list[WhitelistEntryOut]
    total: int
    page: int
    page_size: int
    total_pages: int


# ── Blacklist ───────────────────────────────────────────────────────────────
class BlacklistEntryIn(PolicyEntryBase):
    severity: Severity = "high"
    reason: str = Field(..., min_length=1)


class BlacklistEntryUpdate(BaseModel):
    name_pattern: str | None = Field(default=None, min_length=1, max_length=500)
    publisher_pattern: str | None = Field(default=None, max_length=255)
    version_constraint: str | None = Field(default=None, max_length=100)
    notes: str | None = None
    severity: Severity | None = None
    reason: str | None = Field(default=None, min_length=1)


class BlacklistEntryOut(PolicyEntryBase):
    model_config = ConfigDict(from_attributes=True)
    uuid: uuid_lib.UUID
    severity: Severity
    reason: str
    created_at: datetime
    updated_at: datetime


class BlacklistList(BaseModel):
    items: list[BlacklistEntryOut]
    total: int
    page: int
    page_size: int
    total_pages: int


# ── Bulk import ──────────────────────────────────────────────────────────────
class BulkImportError(BaseModel):
    row: int  # 1-based data row number (header is row 0)
    message: str


class BulkImportResult(BaseModel):
    imported: int
    skipped: int
    errors: list[BulkImportError] = Field(default_factory=list)


# ── Policy mode ──────────────────────────────────────────────────────────────
class PolicyOut(BaseModel):
    whitelist_mode: bool
    flag_unsigned: bool


class PolicyUpdate(BaseModel):
    whitelist_mode: bool | None = None
    flag_unsigned: bool | None = None
