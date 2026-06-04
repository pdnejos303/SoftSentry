"""Pydantic schemas for the Module 7 overview dashboard endpoints."""

from __future__ import annotations

import uuid as uuid_lib
from datetime import date

from pydantic import BaseModel


class SeverityCount(BaseModel):
    severity: str
    count: int


class Overview(BaseModel):
    """Top-row KPI counts for the overview page."""

    machines_total: int
    agents_online: int
    software_unique: int
    vuln_critical: int
    vuln_high: int
    vuln_medium: int
    vuln_low: int


class RiskScoreItem(BaseModel):
    machine_uuid: uuid_lib.UUID
    hostname: str
    risk_score: float
    color: str


class RiskScoreList(BaseModel):
    items: list[RiskScoreItem]


class VulnTrendPoint(BaseModel):
    date: date
    critical: int
    high: int
    medium: int
    low: int


class VulnTrend(BaseModel):
    period_days: int
    points: list[VulnTrendPoint]


class RiskBreakdown(BaseModel):
    """Per-machine risk card (spec 7.2)."""

    risk_score: float
    color: str
    unsigned: int
    unauthorized: int
    cve_critical: int
    cve_high: int
    cve_medium: int
    cve_low: int
