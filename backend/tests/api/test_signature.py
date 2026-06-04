"""Module 3 — digital-signature alerts + signature endpoints end-to-end.

Drives the real endpoints: scan ingest (with signature metadata) → signature
alert engine (spec 3.4) + the /signatures listing/detail/stats endpoints.
"""

from __future__ import annotations

from datetime import UTC, date, datetime, timedelta

import pytest
from app.core.security import hash_password
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
        "/api/v1/auth/login",
        json={"email": "admin@local", "password": password},
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


def _sw(name: str, version: str, sig: dict | None) -> dict:
    entry: dict = {
        "name": name,
        "version": version,
        "publisher": "ACME",
        "source": "registry",
        "arch": "x64",
    }
    if sig is not None:
        entry["signature"] = sig
    return entry


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
        "/api/v1/agents/scans",
        headers={"Authorization": f"Bearer {token}"},
        json=payload,
    )


async def _alerts(client, access: str, **params) -> list[dict]:
    r = await client.get(
        "/api/v1/alerts", headers={"Authorization": f"Bearer {access}"}, params=params
    )
    assert r.status_code == 200, r.text
    return r.json()["items"]


VALID = {"status": "valid", "signer": "Brave Software", "issuer": "DigiCert"}
UNSIGNED = {"status": "unsigned"}
INVALID = {"status": "invalid", "signer": "Sketchy", "issuer": "Self"}
EXPIRED = {
    "status": "expired",
    "signer": "Old Vendor",
    "cert_valid_to": (date.today() - timedelta(days=60)).isoformat(),
}


@pytest.mark.asyncio
async def test_unsigned_software_creates_medium_alert(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)

    await _post_scan(client, agent, _scan(_sw("Sketchy Tool", "1.0", UNSIGNED)))

    alerts = await _alerts(client, access, type="signature_unsigned")
    assert len(alerts) == 1
    assert alerts[0]["severity"] == "medium"
    assert alerts[0]["software_name"] == "Sketchy Tool"
    assert alerts[0]["status"] == "active"


@pytest.mark.asyncio
async def test_invalid_signature_creates_high_alert(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)

    await _post_scan(client, agent, _scan(_sw("Bad App", "2.0", INVALID)))

    alerts = await _alerts(client, access, type="signature_invalid")
    assert len(alerts) == 1
    assert alerts[0]["severity"] == "high"


@pytest.mark.asyncio
async def test_expired_signature_creates_medium_invalid_alert(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)

    await _post_scan(client, agent, _scan(_sw("Old App", "1.0", EXPIRED)))

    alerts = await _alerts(client, access, type="signature_invalid")
    assert len(alerts) == 1
    assert alerts[0]["severity"] == "medium"


@pytest.mark.asyncio
async def test_valid_signature_creates_no_alert(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)

    await _post_scan(client, agent, _scan(_sw("Brave", "1.6", VALID)))

    alerts = await _alerts(client, access)
    assert [a for a in alerts if a["type"].startswith("signature_")] == []


@pytest.mark.asyncio
async def test_rescan_does_not_duplicate_signature_alert(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)

    await _post_scan(client, agent, _scan(_sw("Sketchy Tool", "1.0", UNSIGNED)))
    await _post_scan(client, agent, _scan(_sw("Sketchy Tool", "1.0", UNSIGNED)))

    alerts = await _alerts(client, access, type="signature_unsigned")
    assert len(alerts) == 1


@pytest.mark.asyncio
async def test_signature_fixed_auto_resolves_alert(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)

    # First unsigned → alert, then a newer signed version replaces it.
    await _post_scan(client, agent, _scan(_sw("App", "1.0", UNSIGNED)))
    await _post_scan(client, agent, _scan(_sw("App", "1.0", VALID)))

    active = await _alerts(client, access, status="active", type="signature_unsigned")
    assert active == []
    resolved = await _alerts(client, access, status="resolved", type="signature_unsigned")
    assert len(resolved) == 1


async def _signatures(client, access: str, **params) -> dict:
    r = await client.get(
        "/api/v1/signatures", headers={"Authorization": f"Bearer {access}"}, params=params
    )
    assert r.status_code == 200, r.text
    return r.json()


@pytest.mark.asyncio
async def test_signatures_list_returns_software_with_status(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)
    await _post_scan(
        client,
        agent,
        _scan(_sw("Brave", "1.6", VALID), _sw("Sketchy Tool", "1.0", UNSIGNED)),
    )

    data = await _signatures(client, access)
    assert data["total"] == 2
    names = {i["name"]: i for i in data["items"]}
    assert names["Brave"]["status"] == "valid"
    assert names["Brave"]["signer"] == "Brave Software"
    assert names["Sketchy Tool"]["status"] == "unsigned"
    assert names["Sketchy Tool"]["machine_hostname"] == "host-1"


@pytest.mark.asyncio
async def test_signatures_list_filters_by_status(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)
    await _post_scan(
        client,
        agent,
        _scan(_sw("Brave", "1.6", VALID), _sw("Sketchy Tool", "1.0", UNSIGNED)),
    )

    data = await _signatures(client, access, status="unsigned")
    assert data["total"] == 1
    assert data["items"][0]["name"] == "Sketchy Tool"


@pytest.mark.asyncio
async def test_signatures_list_multi_status_union(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)
    await _post_scan(
        client,
        agent,
        _scan(
            _sw("Good", "1", VALID),
            _sw("NoSig", "1", UNSIGNED),
            _sw("Bad", "1", INVALID),
        ),
    )

    data = await _signatures(client, access, status="unsigned,invalid")
    assert data["total"] == 2
    assert {i["name"] for i in data["items"]} == {"NoSig", "Bad"}


@pytest.mark.asyncio
async def test_signature_detail_returns_chain(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)
    sig = {
        "status": "valid",
        "signer": "Brave Software",
        "issuer": "DigiCert SHA2",
        "cert_thumbprint": "ABCDEF1234",
        "cert_valid_from": "2024-01-01",
        "cert_valid_to": "2026-12-31",
        "signature_algorithm": "SHA256-RSA",
        "chain": [
            {"subject": "Brave Software", "issuer": "DigiCert SHA2"},
            {"subject": "DigiCert SHA2", "issuer": "DigiCert Root"},
            {"subject": "DigiCert Root", "issuer": "DigiCert Root"},
        ],
    }
    await _post_scan(client, agent, _scan(_sw("Brave", "1.6", sig)))

    sw_uuid = (await _signatures(client, access))["items"][0]["software_uuid"]
    r = await client.get(
        f"/api/v1/signatures/{sw_uuid}", headers={"Authorization": f"Bearer {access}"}
    )
    assert r.status_code == 200, r.text
    body = r.json()
    assert body["status"] == "valid"
    assert body["cert_thumbprint"] == "ABCDEF1234"
    assert len(body["chain"]) == 3
    assert body["software_name"] == "Brave"


@pytest.mark.asyncio
async def test_signature_detail_lists_issues_for_expired(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)
    await _post_scan(client, agent, _scan(_sw("Old App", "1.0", EXPIRED)))

    sw_uuid = (await _signatures(client, access))["items"][0]["software_uuid"]
    r = await client.get(
        f"/api/v1/signatures/{sw_uuid}", headers={"Authorization": f"Bearer {access}"}
    )
    body = r.json()
    assert body["status"] == "expired"
    assert any("expired" in issue.lower() for issue in body["issues"])


@pytest.mark.asyncio
async def test_signature_detail_404_for_unknown_software(client, session):
    access = await _admin_access(client, session)
    import uuid as uuid_lib

    r = await client.get(
        f"/api/v1/signatures/{uuid_lib.uuid4()}",
        headers={"Authorization": f"Bearer {access}"},
    )
    assert r.status_code == 404


@pytest.mark.asyncio
async def test_signature_stats_distribution(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)
    await _post_scan(
        client,
        agent,
        _scan(
            _sw("A", "1", VALID),
            _sw("B", "1", VALID),
            _sw("C", "1", UNSIGNED),
            _sw("D", "1", INVALID),
        ),
    )

    r = await client.get(
        "/api/v1/dashboard/signature-stats", headers={"Authorization": f"Bearer {access}"}
    )
    assert r.status_code == 200, r.text
    body = r.json()
    assert body["total"] == 4
    counts = {d["status"]: d["count"] for d in body["distribution"]}
    assert counts["valid"] == 2
    assert counts["unsigned"] == 1
    assert counts["invalid"] == 1


@pytest.mark.asyncio
async def test_flag_unsigned_disabled_suppresses_unsigned_but_not_invalid(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)
    # Turn off unsigned flagging (admin found it noisy); invalid still alerts.
    r = await client.patch(
        "/api/v1/policy",
        headers={"Authorization": f"Bearer {access}"},
        json={"flag_unsigned": False},
    )
    assert r.status_code == 200, r.text

    await _post_scan(
        client,
        agent,
        _scan(_sw("Unsigned Tool", "1.0", UNSIGNED), _sw("Bad App", "2.0", INVALID)),
    )

    assert await _alerts(client, access, type="signature_unsigned") == []
    assert len(await _alerts(client, access, type="signature_invalid")) == 1
