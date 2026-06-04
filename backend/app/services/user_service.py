"""User management business logic (Module 9 — spec 9.4 / 9.5).

Soft-delete + disable rely on the DB re-checks already in
``get_current_user`` / ``rotate_refresh`` (both filter ``is_active`` and
``deleted_at``), so a disabled or deleted user's existing access token fails on
its next request and refresh fails immediately — no explicit Redis session
revocation is needed here.
"""

from __future__ import annotations

import uuid as uuid_lib
from datetime import UTC, datetime
from typing import TYPE_CHECKING

from sqlalchemy import ColumnElement, func, select

from app.core.password_policy import generate_password, validate_password
from app.core.security import hash_password
from app.models.user import User

if TYPE_CHECKING:
    from sqlalchemy.ext.asyncio import AsyncSession


class UserServiceError(Exception):
    """Base for user-management domain errors."""


class EmailAlreadyExists(UserServiceError):
    pass


class UserNotFound(UserServiceError):
    pass


class CannotDeleteSelf(UserServiceError):
    pass


class LastAdminProtected(UserServiceError):
    """Refuses to demote/disable/delete the final remaining admin."""


async def _active_admin_count(session: AsyncSession) -> int:
    stmt = select(func.count()).select_from(User).where(
        User.role == "admin",
        User.is_active.is_(True),
        User.deleted_at.is_(None),
    )
    return (await session.execute(stmt)).scalar_one()


async def get_by_uuid(session: AsyncSession, user_uuid: uuid_lib.UUID) -> User | None:
    return (
        await session.execute(
            select(User).where(User.uuid == user_uuid, User.deleted_at.is_(None))
        )
    ).scalar_one_or_none()


async def _email_taken(session: AsyncSession, email: str) -> bool:
    stmt = select(User.id).where(
        func.lower(User.email) == email.lower(), User.deleted_at.is_(None)
    )
    return (await session.execute(stmt)).first() is not None


async def list_users(
    session: AsyncSession,
    *,
    q: str | None = None,
    role: str | None = None,
    is_active: bool | None = None,
    page: int = 1,
    page_size: int = 50,
) -> tuple[list[User], int]:
    stmt = select(User).where(User.deleted_at.is_(None))
    count_stmt = select(func.count()).select_from(User).where(User.deleted_at.is_(None))
    conds: list[ColumnElement[bool]] = []
    if q:
        like = f"%{q.lower()}%"
        conds.append(
            func.lower(User.email).like(like) | func.lower(User.full_name).like(like)
        )
    if role:
        conds.append(User.role == role)
    if is_active is not None:
        conds.append(User.is_active.is_(is_active))
    for c in conds:
        stmt = stmt.where(c)
        count_stmt = count_stmt.where(c)

    total = (await session.execute(count_stmt)).scalar_one()
    stmt = (
        stmt.order_by(User.created_at.desc()).offset((page - 1) * page_size).limit(page_size)
    )
    users = list((await session.execute(stmt)).scalars().all())
    return users, total


async def create_user(
    session: AsyncSession,
    *,
    email: str,
    full_name: str,
    role: str,
    password: str | None = None,
) -> tuple[User, str]:
    """Create a user. If ``password`` is None, one is generated.

    Returns ``(user, plaintext_password)`` — the plaintext is shown once to the
    admin and never stored.
    """
    email = email.strip().lower()
    if await _email_taken(session, email):
        raise EmailAlreadyExists(email)
    plaintext = password or generate_password()
    validate_password(plaintext)
    user = User(
        email=email,
        full_name=full_name.strip(),
        role=role,
        password_hash=hash_password(plaintext),
        is_active=True,
    )
    session.add(user)
    await session.flush()
    return user, plaintext


async def update_user(
    session: AsyncSession,
    *,
    user: User,
    full_name: str | None = None,
    role: str | None = None,
    is_active: bool | None = None,
) -> dict[str, dict[str, object]]:
    """Apply changes in place; returns a {field: {before, after}} change map.

    Guards the last-admin invariant when demoting or disabling an admin.
    """
    changes: dict[str, dict[str, object]] = {}

    demoting = role is not None and role != "admin" and user.role == "admin"
    disabling = is_active is False and user.is_active and user.role == "admin"
    if (demoting or disabling) and await _active_admin_count(session) <= 1:
        raise LastAdminProtected()

    if full_name is not None and full_name.strip() != user.full_name:
        changes["full_name"] = {"before": user.full_name, "after": full_name.strip()}
        user.full_name = full_name.strip()
    if role is not None and role != user.role:
        changes["role"] = {"before": user.role, "after": role}
        user.role = role
    if is_active is not None and is_active != user.is_active:
        changes["is_active"] = {"before": user.is_active, "after": is_active}
        user.is_active = is_active

    await session.flush()
    return changes


async def soft_delete_user(
    session: AsyncSession,
    *,
    user: User,
    acting_user_id: int,
) -> None:
    if user.id == acting_user_id:
        raise CannotDeleteSelf()
    if user.role == "admin" and await _active_admin_count(session) <= 1:
        raise LastAdminProtected()
    user.deleted_at = datetime.now(tz=UTC)
    user.is_active = False
    await session.flush()


async def reset_password(session: AsyncSession, *, user: User) -> str:
    """Generate a new random password, hash it, return the plaintext once."""
    plaintext = generate_password()
    user.password_hash = hash_password(plaintext)
    await session.flush()
    return plaintext
