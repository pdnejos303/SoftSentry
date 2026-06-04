"""Pydantic schemas for reporting + scheduling (Module 8)."""

from __future__ import annotations

import uuid as uuid_lib
from datetime import datetime
from typing import Any, Literal

from pydantic import BaseModel, Field, model_validator

ReportType = Literal["org_summary", "machine_detail"]
ReportFormat = Literal["pdf", "csv"]
ReportStatus = Literal["queued", "running", "completed", "failed"]


class ReportGenerateRequest(BaseModel):
    type: ReportType
    format: ReportFormat = "pdf"
    # Required when type == machine_detail.
    machine_uuid: uuid_lib.UUID | None = None
    params: dict[str, Any] = Field(default_factory=dict)

    @model_validator(mode="after")
    def _machine_required(self) -> ReportGenerateRequest:
        if self.type == "machine_detail" and self.machine_uuid is None:
            raise ValueError("machine_uuid is required for machine_detail reports")
        return self


class ReportOut(BaseModel):
    uuid: uuid_lib.UUID
    type: ReportType
    format: ReportFormat
    status: ReportStatus
    machine_uuid: uuid_lib.UUID | None = None
    file_size: int | None = None
    error_message: str | None = None
    generated_by: str | None = None  # user email; null when scheduled
    schedule_uuid: uuid_lib.UUID | None = None
    created_at: datetime
    completed_at: datetime | None = None
    expires_at: datetime | None = None


class ReportList(BaseModel):
    items: list[ReportOut]
    total: int
    page: int
    page_size: int
    total_pages: int


class ReportQueued(BaseModel):
    uuid: uuid_lib.UUID
    status: ReportStatus


# ── Schedules (spec 8.5) ─────────────────────────────────────────────────────


class ScheduleIn(BaseModel):
    name: str = Field(..., min_length=1, max_length=255)
    type: ReportType
    format: ReportFormat = "pdf"
    machine_uuid: uuid_lib.UUID | None = None
    cron: str = Field(..., min_length=1, max_length=50)
    recipients: list[str] = Field(default_factory=list)
    is_active: bool = True
    params: dict[str, Any] = Field(default_factory=dict)

    @model_validator(mode="after")
    def _machine_required(self) -> ScheduleIn:
        if self.type == "machine_detail" and self.machine_uuid is None:
            raise ValueError("machine_uuid is required for machine_detail schedules")
        return self


class ScheduleUpdate(BaseModel):
    name: str | None = Field(default=None, min_length=1, max_length=255)
    format: ReportFormat | None = None
    cron: str | None = Field(default=None, min_length=1, max_length=50)
    recipients: list[str] | None = None
    is_active: bool | None = None
    params: dict[str, Any] | None = None


class ScheduleOut(BaseModel):
    uuid: uuid_lib.UUID
    name: str
    type: ReportType
    format: ReportFormat
    machine_uuid: uuid_lib.UUID | None = None
    cron: str
    recipients: list[str]
    is_active: bool
    last_run_at: datetime | None = None
    next_run_at: datetime | None = None
    upcoming_runs: list[datetime] = Field(default_factory=list)
    created_at: datetime


class ScheduleList(BaseModel):
    items: list[ScheduleOut]
    total: int
