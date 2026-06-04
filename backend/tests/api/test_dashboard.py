"""Module 7 — overview KPIs, top-N risk scores, vuln trend, and the per-machine
risk breakdown, end-to-end through the scan-ingest pipeline."""

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
            role="admin",
            is_active=True,
        )
    )
    await session.commit()
    r = await client.post(
        "/api/v1/auth/login", json={"email": "admin@local", "password": password}
    )
    return r.json()["access_token"]


def _hdr(access: str) -> dict:
    return {"Authorization": f"Bearer {access}"}


async def _enroll_agent(client, access: str, hostname: str) -> str:
    create = await client.post(
        "/api/v1/agents/enrollment-tokens",
        headers=_hdr(access),
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


def _scan(*software: dict) -> dict:
    now = datetime.now(tz=UTC)
    return {
        "started_at": (now - timedelta(seconds=5)).isoformat(),
        "completed_at": now.isoformat(),
        "scan_type": "auto",
        "trigger": "schedule",
        "software": list(software),
    }


def _sw(
    name: str,
    version: str,
    *,
    publisher: str = "Google LLC",
    signature: dict | None = None,
) -> dict:
    item = {
        "name": name,
        "version": version,
        "publisher": publisher,
        "source": "registry",
        "arch": "x64",
    }
    if signature is not None:
        item["signature"] = signature
    return item


async def _post_scan(client, token: str, payload: dict):
    return await client.post("/api/v1/agents/scans", headers=_hdr(token), json=payload)


async def _seed_cve(session, cve_id: str, *, severity: str, affected: list[dict]) -> None:
    now = datetime.now(tz=UTC)
    session.add(
        CveRecord(
            cve_id=cve_id,
            source="nvd",
            published_at=now,
            modified_at=now,
            severity=severity,
            cvss_score=9.8,
            description=f"Test {cve_id}",
            cpe_matches=[],
            affected=affected,
            references=[],
        )
    )
    await session.commit()


_CHROME_AFFECTED = [{"product": "chrome", "vendor": "google", "version_end_excluding": "101.0"}]


# ── Overview KPIs ─────────────────────────────────────────────────────────────
@pytest.mark.asyncio
async def test_overview_counts(client, session):
    access = await _admin_access(client, session)
    a1 = await _enroll_agent(client, access, "host-1")
    a2 = await _enroll_agent(client, access, "host-2")
    await _post_scan(client, a1, _scan(_sw("Google Chrome", "100.0"), _sw("Slack", "5.0")))
    await _post_scan(client, a2, _scan(_sw("Google Chrome", "100.0")))

    r = await client.get("/api/v1/dashboard/overview", headers=_hdr(access))
    assert r.status_code == 200, r.text
    body = r.json()
    assert body["machines_total"] == 2
    assert body["agents_online"] == 2
    # Distinct names across the fleet: Chrome + Slack.
    assert body["software_unique"] == 2


@pytest.mark.asyncio
async def test_overview_requires_auth(client, session):
    r = await client.get("/api/v1/dashboard/overview")
    assert r.status_code == 401


@pytest.mark.asyncio
async def test_overview_vuln_severity(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access, "host-1")
    await _seed_cve(session, "CVE-2024-CRIT", severity="critical", affected=_CHROME_AFFECTED)
    await _post_scan(client, agent, _scan(_sw("Google Chrome", "100.0")))

    body = (await client.get("/api/v1/dashboard/overview", headers=_hdr(access))).json()
    assert body["vuln_critical"] == 1
    assert body["vuln_high"] == 0


# ── Risk scores ───────────────────────────────────────────────────────────────
@pytest.mark.asyncio
async def test_risk_scores_ranked(client, session):
    access = await _admin_access(client, session)
    # host-high: unsigned (x1) + critical CVE (x5) = 6.
    high = await _enroll_agent(client, access, "host-high")
    # host-low: just unsigned = 1.
    low = await _enroll_agent(client, access, "host-low")
    await _seed_cve(session, "CVE-2024-CRIT", severity="critical", affected=_CHROME_AFFECTED)

    await _post_scan(
        client,
        high,
        _scan(_sw("Google Chrome", "100.0", signature={"status": "unsigned"})),
    )
    await _post_scan(
        client,
        low,
        _scan(_sw("Slack", "5.0", signature={"status": "unsigned"})),
    )

    r = await client.get("/api/v1/dashboard/risk-scores", headers=_hdr(access))
    assert r.status_code == 200, r.text
    items = r.json()["items"]
    assert [i["hostname"] for i in items] == ["host-high", "host-low"]
    assert items[0]["risk_score"] == 6
    assert items[0]["color"] == "yellow"
    assert items[1]["risk_score"] == 1


@pytest.mark.asyncio
async def test_risk_scores_excludes_zero(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access, "clean-host")
    # Signed, no CVE → risk 0 → excluded from the top-N list.
    await _post_scan(
        client, agent, _scan(_sw("Google Chrome", "100.0", signature={"status": "valid"}))
    )

    items = (await client.get("/api/v1/dashboard/risk-scores", headers=_hdr(access))).json()[
        "items"
    ]
    assert items == []


# ── Per-machine risk breakdown ────────────────────────────────────────────────
@pytest.mark.asyncio
async def test_machine_risk_breakdown(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access, "host-1")
    await _seed_cve(session, "CVE-2024-CRIT", severity="critical", affected=_CHROME_AFFECTED)
    await _post_scan(
        client,
        agent,
        _scan(_sw("Google Chrome", "100.0", signature={"status": "unsigned"})),
    )

    machine = (await client.get("/api/v1/machines", headers=_hdr(access))).json()["items"][0]
    r = await client.get(f"/api/v1/machines/{machine['uuid']}/risk", headers=_hdr(access))
    assert r.status_code == 200, r.text
    body = r.json()
    assert body["unsigned"] == 1
    assert body["cve_critical"] == 1
    assert body["risk_score"] == 6
    assert body["color"] == "yellow"


@pytest.mark.asyncio
async def test_machine_detail_carries_risk(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access, "host-1")
    await _seed_cve(session, "CVE-2024-CRIT", severity="critical", affected=_CHROME_AFFECTED)
    await _post_scan(
        client,
        agent,
        _scan(_sw("Google Chrome", "100.0", signature={"status": "unsigned"})),
    )

    machine = (await client.get("/api/v1/machines", headers=_hdr(access))).json()["items"][0]
    detail = (
        await client.get(f"/api/v1/machines/{machine['uuid']}", headers=_hdr(access))
    ).json()
    assert detail["risk_score"] == 6
    assert detail["vulnerability_count"]["critical"] == 1


# ── Vuln trend ────────────────────────────────────────────────────────────────
@pytest.mark.asyncio
async def test_vuln_trend_buckets_today(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access, "host-1")
    await _seed_cve(session, "CVE-2024-CRIT", severity="critical", affected=_CHROME_AFFECTED)
    await _post_scan(client, agent, _scan(_sw("Google Chrome", "100.0")))

    r = await client.get(
        "/api/v1/dashboard/charts/vuln-trend", headers=_hdr(access), params={"period": "30d"}
    )
    assert r.status_code == 200, r.text
    body = r.json()
    assert body["period_days"] == 30
    assert len(body["points"]) == 30
    today = body["points"][-1]
    assert today["critical"] == 1
    # Earlier days are zero-filled.
    assert body["points"][0]["critical"] == 0
