"""Module 8 — report generation, download, and schedule CRUD."""

from __future__ import annotations

from datetime import UTC, datetime, timedelta

import pytest
from app.core.security import hash_password
from app.models.user import User
from app.services import report_service, schedule_service

_PW = "TestPass!2026Aa"


@pytest.fixture(autouse=True)
def _stub_enqueue(monkeypatch) -> None:
    """No live arq/redis in tests — the queue path is exercised separately.
    Stub the enqueue so generation endpoints return immediately."""

    async def _noop(_report_uuid) -> bool:
        return True

    monkeypatch.setattr("app.api.v1.reports.enqueue_report", _noop)


async def _user(session, email: str, role: str) -> None:
    session.add(
        User(
            email=email,
            password_hash=hash_password(_PW),
            full_name=email,
            role=role,
            is_active=True,
        )
    )
    await session.commit()


async def _login(client, email: str) -> str:
    r = await client.post("/api/v1/auth/login", json={"email": email, "password": _PW})
    return r.json()["access_token"]


def _hdr(access: str) -> dict:
    return {"Authorization": f"Bearer {access}"}


async def _admin(client, session) -> str:
    await _user(session, "admin@local", "dev")
    return await _login(client, "admin@local")


async def _enroll_and_scan(client, access: str, hostname: str) -> None:
    create = await client.post(
        "/api/v1/agents/enrollment-tokens", headers=_hdr(access), json={"expires_in_seconds": 3600}
    )
    enroll = await client.post(
        "/api/v1/agents/enroll",
        json={
            "enrollment_token": create.json()["token"],
            "hostname": hostname,
            "os": "windows",
            "os_version": "11.0",
            "arch": "amd64",
            "agent_version": "0.1.0",
        },
    )
    token = enroll.json()["agent_token"]
    now = datetime.now(tz=UTC)
    await client.post(
        "/api/v1/agents/scans",
        headers=_hdr(token),
        json={
            "started_at": (now - timedelta(seconds=5)).isoformat(),
            "completed_at": now.isoformat(),
            "scan_type": "auto",
            "trigger": "schedule",
            "software": [
                {"name": "Google Chrome", "version": "100.0", "publisher": "Google LLC",
                 "source": "registry", "arch": "x64"}
            ],
        },
    )


# ── Generation + download ────────────────────────────────────────────────────


@pytest.mark.asyncio
async def test_generate_returns_queued(client, session) -> None:
    access = await _admin(client, session)
    r = await client.post(
        "/api/v1/reports/generate",
        headers=_hdr(access),
        json={"type": "org_summary", "format": "csv"},
    )
    assert r.status_code == 202, r.text
    body = r.json()
    assert body["status"] == "queued"
    assert body["uuid"]


@pytest.mark.asyncio
async def test_machine_detail_requires_machine_uuid(client, session) -> None:
    access = await _admin(client, session)
    r = await client.post(
        "/api/v1/reports/generate",
        headers=_hdr(access),
        json={"type": "machine_detail", "format": "csv"},
    )
    assert r.status_code == 422


@pytest.mark.asyncio
async def test_generate_then_download_csv(client, session, tmp_path, monkeypatch) -> None:
    monkeypatch.setattr(report_service.settings, "reports_dir", str(tmp_path))
    access = await _admin(client, session)
    await _enroll_and_scan(client, access, "host-1")

    queued = (
        await client.post(
            "/api/v1/reports/generate",
            headers=_hdr(access),
            json={"type": "org_summary", "format": "csv"},
        )
    ).json()
    report_uuid = queued["uuid"]

    # Run generation directly (no live worker in tests).
    import uuid as uuid_lib

    report = await report_service.get_report(session, uuid_lib.UUID(report_uuid))
    assert report is not None
    await report_service.generate(session, report)
    await session.commit()

    meta = (await client.get(f"/api/v1/reports/{report_uuid}", headers=_hdr(access))).json()
    assert meta["status"] == "completed"
    assert meta["file_size"] > 0

    dl = await client.get(f"/api/v1/reports/{report_uuid}/download", headers=_hdr(access))
    assert dl.status_code == 200, dl.text
    assert dl.headers["content-type"].startswith("text/csv")
    assert "host-1" in dl.content.decode("utf-8-sig")


@pytest.mark.asyncio
async def test_download_before_ready_conflicts(client, session) -> None:
    access = await _admin(client, session)
    queued = (
        await client.post(
            "/api/v1/reports/generate",
            headers=_hdr(access),
            json={"type": "org_summary", "format": "csv"},
        )
    ).json()
    r = await client.get(f"/api/v1/reports/{queued['uuid']}/download", headers=_hdr(access))
    assert r.status_code == 409


@pytest.mark.asyncio
async def test_list_and_delete_report(client, session) -> None:
    access = await _admin(client, session)
    queued = (
        await client.post(
            "/api/v1/reports/generate",
            headers=_hdr(access),
            json={"type": "org_summary", "format": "csv"},
        )
    ).json()

    listing = (await client.get("/api/v1/reports", headers=_hdr(access))).json()
    assert listing["total"] == 1
    assert listing["items"][0]["generated_by"] == "admin@local"

    d = await client.delete(f"/api/v1/reports/{queued['uuid']}", headers=_hdr(access))
    assert d.status_code == 204
    assert (await client.get("/api/v1/reports", headers=_hdr(access))).json()["total"] == 0


# ── Schedules ─────────────────────────────────────────────────────────────────


@pytest.mark.asyncio
async def test_create_schedule_computes_next_run(client, session) -> None:
    access = await _admin(client, session)
    r = await client.post(
        "/api/v1/reports/schedules",
        headers=_hdr(access),
        json={"name": "Weekly", "type": "org_summary", "format": "pdf", "cron": "0 9 * * MON"},
    )
    assert r.status_code == 201, r.text
    body = r.json()
    assert body["next_run_at"] is not None
    assert len(body["upcoming_runs"]) == 3


@pytest.mark.asyncio
async def test_create_schedule_rejects_bad_cron(client, session) -> None:
    access = await _admin(client, session)
    r = await client.post(
        "/api/v1/reports/schedules",
        headers=_hdr(access),
        json={"name": "Bad", "type": "org_summary", "cron": "not a cron"},
    )
    assert r.status_code == 422


@pytest.mark.asyncio
async def test_schedule_create_forbidden_for_viewer(client, session) -> None:
    await _user(session, "viewer@local", "viewer")
    access = await _login(client, "viewer@local")
    r = await client.post(
        "/api/v1/reports/schedules",
        headers=_hdr(access),
        json={"name": "X", "type": "org_summary", "cron": "0 9 * * *"},
    )
    assert r.status_code == 403


@pytest.mark.asyncio
async def test_pause_schedule_clears_next_run(client, session) -> None:
    access = await _admin(client, session)
    created = (
        await client.post(
            "/api/v1/reports/schedules",
            headers=_hdr(access),
            json={"name": "Daily", "type": "org_summary", "cron": "0 9 * * *"},
        )
    ).json()
    patched = (
        await client.patch(
            f"/api/v1/reports/schedules/{created['uuid']}",
            headers=_hdr(access),
            json={"is_active": False},
        )
    ).json()
    assert patched["is_active"] is False
    assert patched["next_run_at"] is None


@pytest.mark.asyncio
async def test_run_now_queues_report(client, session) -> None:
    access = await _admin(client, session)
    created = (
        await client.post(
            "/api/v1/reports/schedules",
            headers=_hdr(access),
            json={"name": "Daily", "type": "org_summary", "format": "csv", "cron": "0 9 * * *"},
        )
    ).json()
    r = await client.post(
        f"/api/v1/reports/schedules/{created['uuid']}/run-now", headers=_hdr(access)
    )
    assert r.status_code == 202, r.text
    assert r.json()["status"] == "queued"
    # The report links back to the schedule.
    report = (await client.get(f"/api/v1/reports/{r.json()['uuid']}", headers=_hdr(access))).json()
    assert report["schedule_uuid"] == created["uuid"]


@pytest.mark.asyncio
async def test_delete_schedule(client, session) -> None:
    access = await _admin(client, session)
    created = (
        await client.post(
            "/api/v1/reports/schedules",
            headers=_hdr(access),
            json={"name": "Daily", "type": "org_summary", "cron": "0 9 * * *"},
        )
    ).json()
    d = await client.delete(
        f"/api/v1/reports/schedules/{created['uuid']}", headers=_hdr(access)
    )
    assert d.status_code == 204
    listing = (await client.get("/api/v1/reports/schedules", headers=_hdr(access))).json()
    assert listing["total"] == 0


@pytest.mark.asyncio
async def test_due_schedules_picks_up_past_due(client, session) -> None:
    access = await _admin(client, session)
    await client.post(
        "/api/v1/reports/schedules",
        headers=_hdr(access),
        json={"name": "Daily", "type": "org_summary", "cron": "0 9 * * *"},
    )
    # Force the single schedule's next_run_at into the past.
    schedules = await schedule_service.list_schedules(session)
    schedules[0].next_run_at = datetime.now(tz=UTC) - timedelta(minutes=1)
    await session.commit()

    due = await schedule_service.due_schedules(session, datetime.now(tz=UTC))
    assert len(due) == 1
    before = due[0].next_run_at
    schedule_service.mark_ran(due[0], datetime.now(tz=UTC))
    assert due[0].next_run_at > before
