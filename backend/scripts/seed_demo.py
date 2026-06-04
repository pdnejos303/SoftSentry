"""Seed demo inventory so the dashboard/API has something to show immediately.

Creates a few "enrolled" machines and ingests realistic software scans through
the real scan pipeline (``scan_service.process_scan``), so software_records,
signatures, scans and history all get populated exactly as a live agent would.

Run from the backend/ dir using the committed venv:

    .venv\\Scripts\\python.exe -m scripts.seed_demo
    .venv\\Scripts\\python.exe -m scripts.seed_demo --reset   # wipe local DB first

Targets the same local SQLite file as run_local (softsentry_local.db) and seeds
the admin user, so afterwards you can `run-local.ps1` and log in to see data.
Honors the project-root .env for the admin credentials.
"""

from __future__ import annotations

import argparse
import asyncio
import os
import sys
from datetime import UTC, datetime, timedelta
from pathlib import Path

# allow `python scripts/seed_demo.py` as well as `-m scripts.seed_demo`
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from scripts.run_local import _configure_env, _InMemoryRedis

DEMO_MACHINES = [
    {
        "hostname": "DESKTOP-FINANCE-01",
        "os": "windows",
        "os_version": "10.0.22631",
        "arch": "amd64",
        "tags": ["finance", "hq"],
        "software": [
            ("Google Chrome", "120.0.6099.130", "Google LLC", "registry", "valid"),
            ("7-Zip", "19.00", "Igor Pavlov", "registry", "valid"),
            ("OpenSSL", "1.1.1", "OpenSSL", "registry", "unsigned"),
            ("Notepad++", "8.4.1", "Notepad++ Team", "registry", "valid"),
            ("Microsoft Excel", "16.0.17328", "Microsoft Corporation", "registry", "valid"),
        ],
    },
    {
        "hostname": "DESKTOP-HR-02",
        "os": "windows",
        "os_version": "10.0.22631",
        "arch": "amd64",
        "tags": ["hr"],
        "software": [
            ("Google Chrome", "124.0.6367.91", "Google LLC", "registry", "valid"),
            ("7-Zip", "22.01", "Igor Pavlov", "registry", "valid"),
            ("Adobe Acrobat Reader", "23.001.20174", "Adobe Inc.", "registry", "valid"),
            ("Slack", "4.36.140", "Slack Technologies", "registry", "valid"),
        ],
    },
    {
        "hostname": "MBP-DESIGN-03",
        "os": "macos",
        "os_version": "14.4",
        "arch": "arm64",
        "tags": ["design"],
        "software": [
            ("Google Chrome", "119.0.6045.199", "Google LLC", "appstore", "valid"),
            ("Mozilla Firefox", "110.0", "Mozilla", "plist", "valid"),
            ("VLC media player", "3.0.16", "VideoLAN", "plist", "unsigned"),
            ("Figma", "116.5.18", "Figma, Inc.", "appstore", "valid"),
        ],
    },
]


async def _seed() -> None:
    from app.core import redis as redis_module
    from app.core.db import SessionLocal, engine
    from app.core.security import hash_password
    from app.models import Base, Machine
    from app.schemas.scans import ScanIn, SignatureIn, SoftwareIn
    from app.seed import seed_admin
    from app.services import scan_service

    redis_module._redis = _InMemoryRedis()  # type: ignore[assignment]

    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    await seed_admin()

    now = datetime.now(tz=UTC)
    async with SessionLocal() as session:
        for idx, spec in enumerate(DEMO_MACHINES):
            machine = Machine(
                hostname=spec["hostname"],
                os=spec["os"],
                os_version=spec["os_version"],
                arch=spec["arch"],
                agent_version="1.0.0",
                agent_token_hash=hash_password(f"demo-agent-token-{idx}"),
                enrolled_at=now - timedelta(days=7),
                last_seen_at=now,
                status="online",
                tags=spec["tags"],
            )
            session.add(machine)
            await session.flush()

            payload = ScanIn(
                started_at=now,
                completed_at=now,
                scan_type="auto",
                trigger="enroll",
                software=[
                    SoftwareIn(
                        name=name,
                        version=version,
                        publisher=publisher,
                        source=source,
                        signature=SignatureIn(status=sig_status, signer=publisher),
                    )
                    for name, version, publisher, source, sig_status in spec["software"]
                ],
            )
            await scan_service.process_scan(session=session, machine=machine, payload=payload)
        await session.commit()

    await engine.dispose()

    n_machines = len(DEMO_MACHINES)
    n_software = sum(len(m["software"]) for m in DEMO_MACHINES)
    print(f"\n✓ seeded {n_machines} machines, {n_software} software instances")
    print("  run the backend (run-local.ps1) and log in to see them in /docs or the dashboard.")


def main() -> None:
    parser = argparse.ArgumentParser(description="Seed SoftSentry demo inventory.")
    parser.add_argument("--reset", action="store_true", help="Delete the local SQLite DB first.")
    args = parser.parse_args()

    _configure_env(args.reset)
    print(f"→ database: {os.environ['DATABASE_URL']}")
    asyncio.run(_seed())


if __name__ == "__main__":
    main()
