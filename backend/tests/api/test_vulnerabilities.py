"""Module 5 — CVE matching + vulnerability endpoints end-to-end.

Drives the real scan-ingest pipeline (which runs the vuln engine inline) plus
the /vulnerabilities, /cve, dashboard summary, and dismiss endpoints. CVEs are
seeded directly into `cve_records` (the NVD fetch is unit-tested separately).
"""

from __future__ import annotations

from datetime import UTC, datetime, timedelta

import pytest
from app.core.security import hash_password
from app.models.cve import CveRecord
from app.models.user import User


async def _admin_access(client, session) -> str:
    password = "TestPass!2026Aa"
    session.add(
        User(
            email="admin@local",
            password_hash=hash_password(password),
            full_name="Admin",
            role="dev",
            is_active=True,
        )
    )
    await session.commit()
    r = await client.post("/api/v1/auth/login", json={"email": "admin@local", "password": password})
    return r.json()["access_token"]


async def _viewer_access(client, session) -> str:
    password = "ViewPass!2026Aa"
    session.add(
        User(
            email="viewer@local",
            password_hash=hash_password(password),
            full_name="Viewer",
            role="viewer",
            is_active=True,
        )
    )
    await session.commit()
    r = await client.post(
        "/api/v1/auth/login", json={"email": "viewer@local", "password": password}
    )
    return r.json()["access_token"]


async def _enroll_agent(client, access: str, hostname: str = "host-1") -> str:
    create = await client.post(
        "/api/v1/agents/enrollment-tokens",
        headers={"Authorization": f"Bearer {access}"},
        json={"expires_in_seconds": 3600},
    )
    enroll = await client.post(
        "/api/v1/agents/enroll",
        json={
            "enrollment_token": create.json()["token"],
            "hostname": hostname,
            "os": "windows",
            "os_version": "11.0.26200",
            "arch": "amd64",
            "agent_version": "0.1.0",
        },
    )
    return enroll.json()["agent_token"]


def _sw(name: str, version: str, publisher: str | None = "Google LLC") -> dict:
    return {
        "name": name,
        "version": version,
        "publisher": publisher,
        "source": "registry",
        "arch": "x64",
    }


def _scan(*software: dict) -> dict:
    now = datetime.now(tz=UTC)
    return {
        "started_at": (now - timedelta(seconds=5)).isoformat(),
        "completed_at": now.isoformat(),
        "scan_type": "auto",
        "trigger": "schedule",
        "software": list(software),
    }


async def _post_scan(client, token: str, payload: dict):
    return await client.post(
        "/api/v1/agents/scans", headers={"Authorization": f"Bearer {token}"}, json=payload
    )


async def _seed_cve(
    session,
    cve_id: str,
    *,
    severity: str = "high",
    affected: list[dict],
    cvss: float = 7.5,
) -> None:
    now = datetime.now(tz=UTC)
    session.add(
        CveRecord(
            cve_id=cve_id,
            source="nvd",
            published_at=now,
            modified_at=now,
            severity=severity,
            cvss_score=cvss,
            description=f"Test vulnerability {cve_id}",
            cpe_matches=["cpe:2.3:a:google:chrome:*:*:*:*:*:*:*:*"],
            affected=affected,
            references=["https://nvd.nist.gov/vuln/detail/" + cve_id],
        )
    )
    await session.commit()


_CHROME_AFFECTED = [{"product": "chrome", "vendor": "google", "version_end_excluding": "101.0"}]


async def _vulns(client, access: str, **params) -> dict:
    r = await client.get(
        "/api/v1/vulnerabilities", headers={"Authorization": f"Bearer {access}"}, params=params
    )
    assert r.status_code == 200, r.text
    return r.json()


@pytest.mark.asyncio
async def test_matching_software_produces_vulnerability(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)
    await _seed_cve(session, "CVE-2024-0001", severity="high", affected=_CHROME_AFFECTED)

    await _post_scan(client, agent, _scan(_sw("Google Chrome", "100.0")))

    data = await _vulns(client, access)
    assert data["total"] == 1
    item = data["items"][0]
    assert item["cve_id"] == "CVE-2024-0001"
    assert item["severity"] == "high"
    assert item["software_name"] == "Google Chrome"
    assert item["recommended_version"] == "101.0"
    assert item["match_confidence"] == "high"


@pytest.mark.asyncio
async def test_up_to_date_software_has_no_vulnerability(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)
    await _seed_cve(session, "CVE-2024-0001", affected=_CHROME_AFFECTED)

    await _post_scan(client, agent, _scan(_sw("Google Chrome", "101.0")))

    assert (await _vulns(client, access))["total"] == 0


@pytest.mark.asyncio
async def test_upgrade_resolves_vulnerability(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)
    await _seed_cve(session, "CVE-2024-0001", affected=_CHROME_AFFECTED)

    await _post_scan(client, agent, _scan(_sw("Google Chrome", "100.0")))
    assert (await _vulns(client, access))["total"] == 1
    # Patch it.
    await _post_scan(client, agent, _scan(_sw("Google Chrome", "101.0")))
    assert (await _vulns(client, access))["total"] == 0


@pytest.mark.asyncio
async def test_vendor_mismatch_is_not_a_false_positive(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)
    await _seed_cve(
        session,
        "CVE-2024-EXPR",
        affected=[{"product": "express", "vendor": "openjsf", "version_end_excluding": "5.0"}],
    )

    await _post_scan(client, agent, _scan(_sw("Express", "1.0", publisher="Acme Desktop Inc")))

    assert (await _vulns(client, access))["total"] == 0


@pytest.mark.asyncio
async def test_low_confidence_hidden_by_default(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)
    # No version range + no publisher → low confidence (product-only).
    await _seed_cve(session, "CVE-2024-LOW", affected=[{"product": "chrome", "vendor": "google"}])

    await _post_scan(client, agent, _scan(_sw("Google Chrome", "100.0", publisher=None)))

    assert (await _vulns(client, access))["total"] == 0
    # But visible when asking for low confidence explicitly.
    assert (await _vulns(client, access, min_confidence="low"))["total"] == 1


@pytest.mark.asyncio
async def test_dismiss_hides_and_undismiss_restores(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)
    await _seed_cve(session, "CVE-2024-0001", affected=_CHROME_AFFECTED)
    await _post_scan(client, agent, _scan(_sw("Google Chrome", "100.0")))

    vuln_uuid = (await _vulns(client, access))["items"][0]["uuid"]

    r = await client.post(
        f"/api/v1/vulnerabilities/{vuln_uuid}/dismiss",
        headers={"Authorization": f"Bearer {access}"},
        json={"reason": "Accepted risk — air-gapped host"},
    )
    assert r.status_code == 200, r.text
    assert r.json()["is_dismissed"] is True

    assert (await _vulns(client, access))["total"] == 0
    shown = await _vulns(client, access, show_dismissed=True)
    assert shown["total"] == 1
    assert shown["items"][0]["dismissed_reason"].startswith("Accepted risk")

    r = await client.post(
        f"/api/v1/vulnerabilities/{vuln_uuid}/undismiss",
        headers={"Authorization": f"Bearer {access}"},
    )
    assert r.status_code == 200, r.text
    assert (await _vulns(client, access))["total"] == 1


@pytest.mark.asyncio
async def test_viewer_cannot_dismiss(client, session):
    admin = await _admin_access(client, session)
    viewer = await _viewer_access(client, session)
    agent = await _enroll_agent(client, admin)
    await _seed_cve(session, "CVE-2024-0001", affected=_CHROME_AFFECTED)
    await _post_scan(client, agent, _scan(_sw("Google Chrome", "100.0")))
    vuln_uuid = (await _vulns(client, admin))["items"][0]["uuid"]

    r = await client.post(
        f"/api/v1/vulnerabilities/{vuln_uuid}/dismiss",
        headers={"Authorization": f"Bearer {viewer}"},
        json={"reason": "nope"},
    )
    assert r.status_code == 403


@pytest.mark.asyncio
async def test_critical_cve_raises_and_resolves_alert(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)
    await _seed_cve(
        session, "CVE-2024-CRIT", severity="critical", cvss=9.8, affected=_CHROME_AFFECTED
    )

    await _post_scan(client, agent, _scan(_sw("Google Chrome", "100.0")))

    r = await client.get(
        "/api/v1/alerts",
        headers={"Authorization": f"Bearer {access}"},
        params={"type": "cve_critical"},
    )
    alerts = r.json()["items"]
    assert len(alerts) == 1
    assert alerts[0]["severity"] == "critical"
    assert "CVE-2024-CRIT" in alerts[0]["detail"]

    # Upgrade → alert auto-resolves.
    await _post_scan(client, agent, _scan(_sw("Google Chrome", "101.0")))
    active = await client.get(
        "/api/v1/alerts",
        headers={"Authorization": f"Bearer {access}"},
        params={"type": "cve_critical", "status": "active"},
    )
    assert active.json()["items"] == []


@pytest.mark.asyncio
async def test_vuln_summary_counts_by_severity(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)
    await _seed_cve(session, "CVE-2024-CRIT", severity="critical", affected=_CHROME_AFFECTED)
    await _seed_cve(
        session,
        "CVE-2024-HIGH",
        severity="high",
        affected=[{"product": "firefox", "vendor": "mozilla", "version_end_excluding": "120.0"}],
    )
    await _post_scan(
        client,
        agent,
        _scan(
            _sw("Google Chrome", "100.0"),
            _sw("Firefox", "119.0", publisher="Mozilla Corporation"),
        ),
    )

    r = await client.get(
        "/api/v1/dashboard/vuln-summary", headers={"Authorization": f"Bearer {access}"}
    )
    assert r.status_code == 200, r.text
    body = r.json()
    assert body["total"] == 2
    counts = {d["severity"]: d["count"] for d in body["distribution"]}
    assert counts["critical"] == 1
    assert counts["high"] == 1


@pytest.mark.asyncio
async def test_per_machine_vulnerabilities(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)
    await _seed_cve(session, "CVE-2024-0001", affected=_CHROME_AFFECTED)
    await _post_scan(client, agent, _scan(_sw("Google Chrome", "100.0")))

    machines = await client.get("/api/v1/machines", headers={"Authorization": f"Bearer {access}"})
    machine_uuid = machines.json()["items"][0]["uuid"]

    r = await client.get(
        f"/api/v1/machines/{machine_uuid}/vulnerabilities",
        headers={"Authorization": f"Bearer {access}"},
    )
    assert r.status_code == 200, r.text
    assert r.json()["total"] == 1


@pytest.mark.asyncio
async def test_cve_detail_endpoint(client, session):
    access = await _admin_access(client, session)
    await _seed_cve(session, "CVE-2024-0001", affected=_CHROME_AFFECTED)

    r = await client.get("/api/v1/cve/CVE-2024-0001", headers={"Authorization": f"Bearer {access}"})
    assert r.status_code == 200, r.text
    body = r.json()
    assert body["cve_id"] == "CVE-2024-0001"
    assert body["references"]

    missing = await client.get(
        "/api/v1/cve/CVE-9999-0000", headers={"Authorization": f"Bearer {access}"}
    )
    assert missing.status_code == 404


@pytest.mark.asyncio
async def test_vulnerability_detail_includes_references(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)
    await _seed_cve(session, "CVE-2024-0001", affected=_CHROME_AFFECTED)
    await _post_scan(client, agent, _scan(_sw("Google Chrome", "100.0")))
    vuln_uuid = (await _vulns(client, access))["items"][0]["uuid"]

    r = await client.get(
        f"/api/v1/vulnerabilities/{vuln_uuid}", headers={"Authorization": f"Bearer {access}"}
    )
    assert r.status_code == 200, r.text
    assert r.json()["references"]
