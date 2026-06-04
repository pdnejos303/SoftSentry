"""Per-machine risk scoring (Module 7).

`recompute_machine` runs inline in the scan-ingest transaction (after the
unauthorized / signature / vulnerability engines) so `machines.risk_score` is
always current and the dashboard's "top-N risky machines" widget is a cheap
indexed sort. The score is a weighted sum of the machine's open risk signals:

    risk = unsigned*1 + unauthorized*3
         + cve_critical*5 + cve_high*3 + cve_medium*1 + cve_low*0.5

Counts are read from the source-of-truth tables for the machine's *active*
inventory: signature status for unsigned, open unauthorized/blacklist alerts for
unauthorized, and (non-dismissed, ≥medium-confidence) matched CVEs by severity.
"""

from __future__ import annotations

from typing import TYPE_CHECKING

from sqlalchemy import func, select

from app.models.alert import OPEN_STATUSES, Alert
from app.models.cve import CveRecord, Vulnerability
from app.models.machine import Machine
from app.models.signature import SignatureRecord
from app.models.software import SoftwareRecord
from app.services.unauthorized_service import TYPE_BLACKLISTED, TYPE_UNAUTHORIZED

if TYPE_CHECKING:
    from sqlalchemy.ext.asyncio import AsyncSession

# Severity → weight (spec Module 7 risk formula).
_SEVERITY_WEIGHT = {"critical": 5.0, "high": 3.0, "medium": 1.0, "low": 0.5}
_WEIGHT_UNSIGNED = 1.0
_WEIGHT_UNAUTHORIZED = 3.0

# Color thresholds (spec Module 7): 0 green, 1-10 yellow, 11-30 orange, 31+ red.
COLOR_GREEN = "green"
COLOR_YELLOW = "yellow"
COLOR_ORANGE = "orange"
COLOR_RED = "red"


def compute_risk_score(
    *,
    unsigned: int,
    unauthorized: int,
    cve_critical: int,
    cve_high: int,
    cve_medium: int,
    cve_low: int,
) -> float:
    """Pure weighted-sum risk score. Unit-tested in isolation."""
    return (
        unsigned * _WEIGHT_UNSIGNED
        + unauthorized * _WEIGHT_UNAUTHORIZED
        + cve_critical * _SEVERITY_WEIGHT["critical"]
        + cve_high * _SEVERITY_WEIGHT["high"]
        + cve_medium * _SEVERITY_WEIGHT["medium"]
        + cve_low * _SEVERITY_WEIGHT["low"]
    )


def risk_color(score: float) -> str:
    """Map a score to its threshold color band."""
    if score <= 0:
        return COLOR_GREEN
    if score <= 10:
        return COLOR_YELLOW
    if score <= 30:
        return COLOR_ORANGE
    return COLOR_RED


async def risk_breakdown(session: AsyncSession, machine: Machine) -> dict[str, int]:
    """Per-signal counts for this machine's active inventory (drives the
    per-machine risk card, spec 7.2)."""
    unsigned = (
        await session.execute(
            select(func.count())
            .select_from(SoftwareRecord)
            .join(SignatureRecord, SignatureRecord.id == SoftwareRecord.signature_id)
            .where(
                SoftwareRecord.machine_id == machine.id,
                SoftwareRecord.uninstalled_at.is_(None),
                SignatureRecord.status == "unsigned",
            )
        )
    ).scalar_one()

    unauthorized = (
        await session.execute(
            select(func.count())
            .select_from(Alert)
            .where(
                Alert.machine_id == machine.id,
                Alert.type.in_((TYPE_BLACKLISTED, TYPE_UNAUTHORIZED)),
                Alert.status.in_(OPEN_STATUSES),
            )
        )
    ).scalar_one()

    severity_rows = (
        await session.execute(
            select(CveRecord.severity, func.count(func.distinct(Vulnerability.id)))
            .join(CveRecord, CveRecord.cve_id == Vulnerability.cve_id)
            .join(SoftwareRecord, SoftwareRecord.id == Vulnerability.software_id)
            .where(
                SoftwareRecord.machine_id == machine.id,
                SoftwareRecord.uninstalled_at.is_(None),
                Vulnerability.is_dismissed.is_(False),
                Vulnerability.match_confidence.in_(["high", "medium"]),
            )
            .group_by(CveRecord.severity)
        )
    ).all()
    by_sev = {sev: int(c) for sev, c in severity_rows}

    return {
        "unsigned": int(unsigned),
        "unauthorized": int(unauthorized),
        "cve_critical": by_sev.get("critical", 0),
        "cve_high": by_sev.get("high", 0),
        "cve_medium": by_sev.get("medium", 0),
        "cve_low": by_sev.get("low", 0),
    }


async def recompute_machine(session: AsyncSession, machine: Machine) -> float:
    """Recompute and store `machine.risk_score`. Runs in the caller's
    transaction (caller commits). Returns the new score."""
    counts = await risk_breakdown(session, machine)
    score = compute_risk_score(
        unsigned=counts["unsigned"],
        unauthorized=counts["unauthorized"],
        cve_critical=counts["cve_critical"],
        cve_high=counts["cve_high"],
        cve_medium=counts["cve_medium"],
        cve_low=counts["cve_low"],
    )
    machine.risk_score = score
    return score
