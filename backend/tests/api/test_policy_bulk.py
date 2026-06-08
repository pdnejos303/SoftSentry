"""Module 4.3 — bulk CSV import endpoint (partial accept + dedup)."""

from __future__ import annotations

import pytest
from app.core.security import hash_password
from app.models.user import User

WL_HEADER = "name_pattern,publisher_pattern,version_constraint,notes\n"


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
    r = await client.post(
        "/api/v1/auth/login", json={"email": "admin@local", "password": password}
    )
    return r.json()["access_token"]


@pytest.mark.asyncio
async def test_bulk_import_partial_accept_and_dedup(client, session):
    access = await _admin_access(client, session)
    hdr = {"Authorization": f"Bearer {access}"}

    # Pre-seed one entry so an in-file row duplicates an existing one.
    await client.post("/api/v1/whitelist", headers=hdr, json={"name_pattern": "Existing%"})

    csv_body = (
        WL_HEADER
        + "Microsoft Office %,Microsoft,,suite\n"  # valid
        + "Existing%,,,\n"  # duplicate of seeded entry → skip
        + ",,,no name\n"  # malformed (no name_pattern) → error
        + "Adobe%,Adobe,,design\n"  # valid
    ).encode()

    resp = await client.post(
        "/api/v1/whitelist/bulk",
        headers=hdr,
        files={"file": ("wl.csv", csv_body, "text/csv")},
    )
    assert resp.status_code == 200, resp.text
    body = resp.json()
    assert body["imported"] == 2
    assert body["skipped"] == 1
    # Two error reports: one malformed row, one duplicate.
    assert len(body["errors"]) == 2

    listing = await client.get("/api/v1/whitelist", headers=hdr)
    names = {i["name_pattern"] for i in listing.json()["items"]}
    assert {"Existing%", "Microsoft Office %", "Adobe%"} <= names


@pytest.mark.asyncio
async def test_bulk_import_requires_admin(client, session):
    access = await _admin_access(client, session)
    # Demote: create a viewer and use its token instead.
    viewer_pw = "ViewerPass!2026Aa"
    session.add(
        User(
            email="viewer@local",
            password_hash=hash_password(viewer_pw),
            full_name="Viewer",
            role="viewer",
            is_active=True,
        )
    )
    await session.commit()
    viewer_access = (
        await client.post(
            "/api/v1/auth/login", json={"email": "viewer@local", "password": viewer_pw}
        )
    ).json()["access_token"]

    resp = await client.post(
        "/api/v1/whitelist/bulk",
        headers={"Authorization": f"Bearer {viewer_access}"},
        files={"file": ("wl.csv", (WL_HEADER + "X%,,,\n").encode(), "text/csv")},
    )
    assert resp.status_code == 403
    assert access  # silence unused
