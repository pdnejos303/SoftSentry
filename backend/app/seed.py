"""Seed initial admin user. Run once: `uv run python -m app.seed`."""

from __future__ import annotations

import asyncio
import sys

from sqlalchemy import select

from app.core.config import settings
from app.core.db import SessionLocal
from app.core.security import hash_password
from app.models.user import User


async def _seed_user(
    session,
    *,
    email: str,
    password: str,
    full_name: str,
    role: str,
) -> None:
    existing = (
        await session.execute(select(User).where(User.email == email.lower()))
    ).scalar_one_or_none()

    if existing is not None:
        print(f"{role} already exists: {existing.email} (uuid={existing.uuid})")
        return

    user = User(
        email=email.lower(),
        password_hash=hash_password(password),
        full_name=full_name,
        role=role,
        is_active=True,
    )
    session.add(user)
    await session.commit()
    await session.refresh(user)
    print(f"✓ Seeded {role}: {user.email} (uuid={user.uuid})")


async def seed_admin() -> None:
    async with SessionLocal() as session:
        # Initial admin (limited role: Overview + Machines + Deploy).
        await _seed_user(
            session,
            email=settings.initial_admin_email,
            password=settings.initial_admin_password,
            full_name="Initial Admin",
            role="admin",
        )
        # Dev superuser — sees and does everything. NOTE: this default password is
        # intentionally weak for local/dev convenience; change it before any
        # non-local deployment.
        await _seed_user(
            session,
            email="dev@dev",
            password="01Password",  # noqa: S106  (dev-only seed credential)
            full_name="Developer",
            role="dev",
        )


def main() -> int:
    try:
        asyncio.run(seed_admin())
        return 0
    except Exception as e:
        print(f"Seed failed: {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
