"""CRUD + bulk import for whitelist/blacklist entries and the policy mode toggle."""

from __future__ import annotations

import uuid as uuid_lib
from typing import TYPE_CHECKING, Literal, cast

from sqlalchemy import func, select

from app.models.policy import BlacklistEntry, PolicySettings, WhitelistEntry
from app.schemas.policy import BulkImportError, BulkImportResult
from app.services.policy_csv import parse_policy_csv

if TYPE_CHECKING:
    from sqlalchemy.ext.asyncio import AsyncSession

POLICY_ID = 1

# A policy entry is a row of either list; both share the matched-on columns.
PolicyEntry = WhitelistEntry | BlacklistEntry
PolicyModel = type[WhitelistEntry] | type[BlacklistEntry]


# ── Policy mode ──────────────────────────────────────────────────────────────
async def get_policy(session: AsyncSession) -> PolicySettings:
    policy = await session.get(PolicySettings, POLICY_ID)
    if policy is None:
        policy = PolicySettings(id=POLICY_ID, whitelist_mode=False)
        session.add(policy)
        await session.flush()
    return policy


async def update_policy(
    session: AsyncSession,
    *,
    whitelist_mode: bool | None = None,
    flag_unsigned: bool | None = None,
) -> PolicySettings:
    policy = await get_policy(session)
    if whitelist_mode is not None:
        policy.whitelist_mode = whitelist_mode
    if flag_unsigned is not None:
        policy.flag_unsigned = flag_unsigned
    await session.flush()
    return policy


# ── List ─────────────────────────────────────────────────────────────────────
async def list_entries(
    session: AsyncSession,
    model: PolicyModel,
    *,
    q: str | None,
    page: int,
    page_size: int,
) -> tuple[list[PolicyEntry], int]:
    stmt = select(model)
    count_stmt = select(func.count()).select_from(model)
    if q:
        like = f"%{q}%"
        stmt = stmt.where(model.name_pattern.ilike(like))
        count_stmt = count_stmt.where(model.name_pattern.ilike(like))
    total = (await session.execute(count_stmt)).scalar_one()
    stmt = stmt.order_by(model.created_at.desc()).offset((page - 1) * page_size).limit(page_size)
    items = (await session.execute(stmt)).scalars().all()
    return cast("list[PolicyEntry]", list(items)), total


# ── Create / update / delete ─────────────────────────────────────────────────
async def create_entry(
    session: AsyncSession,
    model: PolicyModel,
    data: dict[str, object],
) -> PolicyEntry:
    entry = model(**data)
    session.add(entry)
    await session.flush()
    return entry


async def get_entry(
    session: AsyncSession,
    model: PolicyModel,
    entry_uuid: uuid_lib.UUID,
) -> PolicyEntry | None:
    result = (
        await session.execute(select(model).where(model.uuid == entry_uuid))
    ).scalar_one_or_none()
    return cast("PolicyEntry | None", result)


async def update_entry(entry: PolicyEntry, data: dict[str, object]) -> None:
    for key, value in data.items():
        setattr(entry, key, value)


async def delete_entry(session: AsyncSession, entry: PolicyEntry) -> None:
    await session.delete(entry)


# ── Bulk import ──────────────────────────────────────────────────────────────
async def bulk_import(
    session: AsyncSession,
    model: PolicyModel,
    data: bytes,
    *,
    kind: str,
) -> BulkImportResult:
    """Parse + insert CSV rows. Rows duplicating an existing (name_pattern,
    publisher_pattern) — or another row in the same file — are skipped and
    reported. Either every valid row is inserted or none (caller commits)."""
    rows, errors = parse_policy_csv(data, kind=cast("Literal['whitelist', 'blacklist']", kind))
    if errors and not rows:
        return BulkImportResult(imported=0, skipped=0, errors=errors)

    # Existing keys for dedup (case-insensitive on name + publisher).
    existing = (await session.execute(select(model.name_pattern, model.publisher_pattern))).all()
    seen: set[tuple[str, str]] = {_dedup_key(n, p) for n, p in existing}

    imported = 0
    skipped = 0
    for i, row in enumerate(rows, start=1):
        key = _dedup_key(row["name_pattern"], row.get("publisher_pattern"))
        if key in seen:
            skipped += 1
            errors.append(BulkImportError(row=i, message="duplicate of an existing entry"))
            continue
        seen.add(key)
        session.add(model(**row))
        imported += 1

    await session.flush()
    return BulkImportResult(imported=imported, skipped=skipped, errors=errors)


def _dedup_key(name: str | None, publisher: str | None) -> tuple[str, str]:
    return ((name or "").strip().lower(), (publisher or "").strip().lower())
