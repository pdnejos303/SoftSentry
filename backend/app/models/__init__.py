"""SQLAlchemy declarative base + all model imports.

`Base.metadata` is consumed by Alembic; importing every model here ensures
auto-generated migrations see the full schema.
"""

from __future__ import annotations

from app.models.agent_config import AgentConfig
from app.models.alert import Alert
from app.models.audit_log import AuditLog
from app.models.base import Base
from app.models.cve import CveRecord, Vulnerability
from app.models.enrollment_token import EnrollmentToken
from app.models.heartbeat import Heartbeat
from app.models.license import License
from app.models.machine import Machine
from app.models.policy import BlacklistEntry, PolicySettings, WhitelistEntry
from app.models.report import Report, ReportSchedule
from app.models.scan import Scan
from app.models.signature import SignatureRecord
from app.models.software import SoftwareHistory, SoftwareRecord
from app.models.user import User

__all__ = [
    "AgentConfig",
    "Alert",
    "AuditLog",
    "Base",
    "BlacklistEntry",
    "CveRecord",
    "EnrollmentToken",
    "Heartbeat",
    "License",
    "Machine",
    "PolicySettings",
    "Report",
    "ReportSchedule",
    "Scan",
    "SignatureRecord",
    "SoftwareHistory",
    "SoftwareRecord",
    "User",
    "Vulnerability",
    "WhitelistEntry",
]
