"""Module 8.1 / 8.2 — Jinja HTML rendering of the report templates.

Only the HTML stage is exercised here (no WeasyPrint), so the test runs on any
platform; PDF byte generation is verified in the Linux container.
"""

from __future__ import annotations

from datetime import UTC, datetime
from typing import Any

from app.services import pdf_service


def _org_ctx() -> dict[str, Any]:
    return {
        "org_name": "Acme Corp",
        "generated_by": "admin@local",
        "generated_at": datetime(2026, 6, 3, 9, 0, tzinfo=UTC),
        "overview": {
            "machines_total": 2,
            "agents_online": 2,
            "software_unique": 5,
            "vuln_critical": 1,
            "vuln_high": 0,
            "vuln_medium": 0,
            "vuln_low": 0,
        },
        "compliance": {
            "compliant": 1,
            "over_used": 0,
            "expired": 0,
            "expiring_90": 0,
            "compliance_rate": 100,
        },
        "risky_machines": [{"hostname": "h1", "risk_score": 6, "color": "yellow"}],
        "trend": {
            "width": 600,
            "height": 140,
            "peak": 1,
            "polylines": {"critical": "0,0", "high": "0,0", "medium": "0,0", "low": "0,0"},
        },
        "machines_by_os": [{"os": "windows", "count": 2, "pct": 100.0}],
        "top_vulnerabilities": [
            {
                "cve_id": "CVE-2024-1",
                "severity": "critical",
                "description": "x",
                "affected_count": 2,
            }
        ],
        "unauthorized": [],
        "unauthorized_total": 0,
    }


def _machine_ctx(*, vulns: list[dict[str, Any]] | None = None) -> dict[str, Any]:
    return {
        "generated_at": datetime(2026, 6, 3, 9, 0, tzinfo=UTC),
        "machine": {
            "hostname": "host-1",
            "os": "windows",
            "os_version": "11.0",
            "arch": "amd64",
            "agent_version": "0.1.0",
            "enrolled_at": datetime(2026, 1, 1, tzinfo=UTC),
            "last_scan_at": datetime(2026, 6, 1, tzinfo=UTC),
            "tags": ["finance"],
            "risk_score": 6,
        },
        "risk_color": "yellow",
        "risk_breakdown": {
            "unsigned": 1,
            "unauthorized": 0,
            "cve_critical": 1,
            "cve_high": 0,
            "cve_medium": 0,
            "cve_low": 0,
        },
        "software": [
            {"name": "Chrome", "version": "100", "publisher": "Google", "signature_status": "valid"}
        ],
        "software_total": 1,
        "vulnerabilities": vulns or [],
        "vulnerabilities_total": len(vulns or []),
        "signature_counts": {"valid": 1},
        "history": [],
        "alerts": [],
        "alerts_total": 0,
    }


def test_org_summary_renders_all_sections() -> None:
    html = pdf_service.render_html("org_summary.html", _org_ctx())
    assert "Acme Corp" in html
    assert "Executive summary" in html
    assert "Top 10 risky machines" in html
    assert "CVE-2024-1" in html
    # Empty unauthorized list shows the empty-state, not a table.
    assert "No active unauthorized-software alerts." in html


def test_machine_report_hides_empty_vuln_section() -> None:
    html = pdf_service.render_html("machine_detail.html", _machine_ctx())
    assert "host-1" in html
    assert "Risk score" in html
    assert "Software inventory (1)" in html
    # No vulnerabilities → the section header is omitted entirely.
    assert "Vulnerabilities (" not in html


def test_machine_report_shows_vuln_section_when_present() -> None:
    vulns = [
        {
            "cve_id": "CVE-2024-9",
            "severity": "high",
            "software_name": "Chrome",
            "software_version": "100",
            "recommended_version": "101",
        }
    ]
    html = pdf_service.render_html("machine_detail.html", _machine_ctx(vulns=vulns))
    assert "Vulnerabilities (1)" in html
    assert "CVE-2024-9" in html
