"""Seed initial admin user. Run once: `uv run python -m app.seed`."""

from __future__ import annotations

import asyncio
import sys

from sqlalchemy import select

from app.core.config import settings
from app.core.db import SessionLocal
from app.core.security import hash_password
from app.models.user import User


async def seed_admin() -> None:
    async with SessionLocal() as session:
        existing = (
            await session.execute(
                select(User).where(User.email == settings.initial_admin_email.lower())
            )
        ).scalar_one_or_none()

        if existing is not None:
            print(f"Admin already exists: {existing.email} (uuid={existing.uuid})")
            return

        admin = User(
            email=settings.initial_admin_email.lower(),
            password_hash=hash_password(settings.initial_admin_password),
            full_name="Initial Admin",
            role="admin",
            is_active=True,
        )
        session.add(admin)
        await session.commit()
        await session.refresh(admin)
        print(f"✓ Seeded admin: {admin.email} (uuid={admin.uuid})")
        print("  Login with password from INITIAL_ADMIN_PASSWORD env var.")


def main() -> int:
    try:
        asyncio.run(seed_admin())
        return 0
    except Exception as e:
        print(f"Seed failed: {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
