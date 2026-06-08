"""Whitelist / blacklist CRUD + bulk import + policy-mode toggle (Module 4)."""

from __future__ import annotations

import uuid as uuid_lib
from typing import Annotated

from fastapi import APIRouter, Depends, File, HTTPException, Query, Request, UploadFile, status

from app.api.v1._audit import audit
from app.core.deps import CurrentUser, DBSession, require_role
from app.models.policy import BlacklistEntry, WhitelistEntry
from app.models.user import User
from app.schemas.policy import (
    BlacklistEntryIn,
    BlacklistEntryOut,
    BlacklistEntryUpdate,
    BlacklistList,
    BulkImportResult,
    PolicyOut,
    PolicyUpdate,
    WhitelistEntryIn,
    WhitelistEntryOut,
    WhitelistEntryUpdate,
    WhitelistList,
)
from app.services import policy_service

# The acting admin (not discarded — needed for audit attribution).
AdminUser = Annotated[User, Depends(require_role("dev"))]
MAX_CSV_BYTES = 5 * 1024 * 1024  # 5MB (spec 4.3)

_ENTRY_FIELDS = (
    "name_pattern", "publisher_pattern", "version_constraint", "notes", "severity", "reason",
)

whitelist_router = APIRouter()
blacklist_router = APIRouter()
policy_router = APIRouter()


def _pages(total: int, page_size: int) -> int:
    return (total + page_size - 1) // page_size


def _entry_snapshot(entry: object) -> dict[str, object]:
    """Non-secret policy-entry fields for audit before/after (blacklist-only
    fields are simply absent on whitelist entries)."""
    snap: dict[str, object] = {}
    for f in _ENTRY_FIELDS:
        if hasattr(entry, f):
            val = getattr(entry, f)
            snap[f] = str(val) if val is not None else None
    return snap


async def _read_csv(file: UploadFile) -> bytes:
    data = await file.read()
    if len(data) > MAX_CSV_BYTES:
        raise HTTPException(status.HTTP_413_REQUEST_ENTITY_TOO_LARGE, "CSV exceeds 5MB limit")
    return data


# ── Whitelist ────────────────────────────────────────────────────────────────
@whitelist_router.get("", response_model=WhitelistList, summary="List whitelist entries")
async def list_whitelist(
    session: DBSession,
    _: CurrentUser,
    q: str | None = None,
    page: Annotated[int, Query(ge=1)] = 1,
    page_size: Annotated[int, Query(ge=1, le=200)] = 50,
) -> WhitelistList:
    items, total = await policy_service.list_entries(
        session, WhitelistEntry, q=q, page=page, page_size=page_size
    )
    return WhitelistList(
        items=[WhitelistEntryOut.model_validate(i) for i in items],
        total=total,
        page=page,
        page_size=page_size,
        total_pages=_pages(total, page_size),
    )


@whitelist_router.post(
    "",
    response_model=WhitelistEntryOut,
    status_code=status.HTTP_201_CREATED,
    summary="Add whitelist entry",
)
async def create_whitelist(
    payload: WhitelistEntryIn, session: DBSession, admin: AdminUser, request: Request
) -> WhitelistEntryOut:
    entry = await policy_service.create_entry(session, WhitelistEntry, payload.model_dump())
    await audit(
        session, request, admin,
        action="whitelist.create", entity_type="whitelist_entry", entity_id=str(entry.uuid),
        after=_entry_snapshot(entry),
    )
    await session.commit()
    return WhitelistEntryOut.model_validate(entry)


@whitelist_router.patch(
    "/{entry_uuid}", response_model=WhitelistEntryOut, summary="Edit whitelist entry"
)
async def update_whitelist(
    entry_uuid: uuid_lib.UUID,
    payload: WhitelistEntryUpdate,
    session: DBSession,
    admin: AdminUser,
    request: Request,
) -> WhitelistEntryOut:
    entry = await policy_service.get_entry(session, WhitelistEntry, entry_uuid)
    if entry is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "Whitelist entry not found")
    before = _entry_snapshot(entry)
    await policy_service.update_entry(entry, payload.model_dump(exclude_unset=True))
    await audit(
        session, request, admin,
        action="whitelist.update", entity_type="whitelist_entry", entity_id=str(entry_uuid),
        before=before, after=_entry_snapshot(entry),
    )
    await session.commit()
    return WhitelistEntryOut.model_validate(entry)


@whitelist_router.delete(
    "/{entry_uuid}", status_code=status.HTTP_204_NO_CONTENT, summary="Delete whitelist entry"
)
async def delete_whitelist(
    entry_uuid: uuid_lib.UUID, session: DBSession, admin: AdminUser, request: Request
) -> None:
    entry = await policy_service.get_entry(session, WhitelistEntry, entry_uuid)
    if entry is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "Whitelist entry not found")
    before = _entry_snapshot(entry)
    await policy_service.delete_entry(session, entry)
    await audit(
        session, request, admin,
        action="whitelist.delete", entity_type="whitelist_entry", entity_id=str(entry_uuid),
        before=before,
    )
    await session.commit()


@whitelist_router.post(
    "/bulk", response_model=BulkImportResult, summary="Bulk import whitelist CSV"
)
async def bulk_whitelist(
    session: DBSession,
    admin: AdminUser,
    request: Request,
    file: Annotated[UploadFile, File()],
) -> BulkImportResult:
    data = await _read_csv(file)
    result = await policy_service.bulk_import(session, WhitelistEntry, data, kind="whitelist")
    await audit(
        session, request, admin,
        action="whitelist.bulk_import", entity_type="whitelist_entry",
        after={
            "imported": result.imported,
            "skipped": result.skipped,
            "errors": len(result.errors),
        },
    )
    await session.commit()
    return result


# ── Blacklist ────────────────────────────────────────────────────────────────
@blacklist_router.get("", response_model=BlacklistList, summary="List blacklist entries")
async def list_blacklist(
    session: DBSession,
    _: CurrentUser,
    q: str | None = None,
    page: Annotated[int, Query(ge=1)] = 1,
    page_size: Annotated[int, Query(ge=1, le=200)] = 50,
) -> BlacklistList:
    items, total = await policy_service.list_entries(
        session, BlacklistEntry, q=q, page=page, page_size=page_size
    )
    return BlacklistList(
        items=[BlacklistEntryOut.model_validate(i) for i in items],
        total=total,
        page=page,
        page_size=page_size,
        total_pages=_pages(total, page_size),
    )


@blacklist_router.post(
    "",
    response_model=BlacklistEntryOut,
    status_code=status.HTTP_201_CREATED,
    summary="Add blacklist entry",
)
async def create_blacklist(
    payload: BlacklistEntryIn, session: DBSession, admin: AdminUser, request: Request
) -> BlacklistEntryOut:
    entry = await policy_service.create_entry(session, BlacklistEntry, payload.model_dump())
    await audit(
        session, request, admin,
        action="blacklist.create", entity_type="blacklist_entry", entity_id=str(entry.uuid),
        after=_entry_snapshot(entry),
    )
    await session.commit()
    return BlacklistEntryOut.model_validate(entry)


@blacklist_router.patch(
    "/{entry_uuid}", response_model=BlacklistEntryOut, summary="Edit blacklist entry"
)
async def update_blacklist(
    entry_uuid: uuid_lib.UUID,
    payload: BlacklistEntryUpdate,
    session: DBSession,
    admin: AdminUser,
    request: Request,
) -> BlacklistEntryOut:
    entry = await policy_service.get_entry(session, BlacklistEntry, entry_uuid)
    if entry is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "Blacklist entry not found")
    before = _entry_snapshot(entry)
    await policy_service.update_entry(entry, payload.model_dump(exclude_unset=True))
    await audit(
        session, request, admin,
        action="blacklist.update", entity_type="blacklist_entry", entity_id=str(entry_uuid),
        before=before, after=_entry_snapshot(entry),
    )
    await session.commit()
    return BlacklistEntryOut.model_validate(entry)


@blacklist_router.delete(
    "/{entry_uuid}", status_code=status.HTTP_204_NO_CONTENT, summary="Delete blacklist entry"
)
async def delete_blacklist(
    entry_uuid: uuid_lib.UUID, session: DBSession, admin: AdminUser, request: Request
) -> None:
    entry = await policy_service.get_entry(session, BlacklistEntry, entry_uuid)
    if entry is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "Blacklist entry not found")
    before = _entry_snapshot(entry)
    await policy_service.delete_entry(session, entry)
    await audit(
        session, request, admin,
        action="blacklist.delete", entity_type="blacklist_entry", entity_id=str(entry_uuid),
        before=before,
    )
    await session.commit()


@blacklist_router.post(
    "/bulk", response_model=BulkImportResult, summary="Bulk import blacklist CSV"
)
async def bulk_blacklist(
    session: DBSession,
    admin: AdminUser,
    request: Request,
    file: Annotated[UploadFile, File()],
) -> BulkImportResult:
    data = await _read_csv(file)
    result = await policy_service.bulk_import(session, BlacklistEntry, data, kind="blacklist")
    await audit(
        session, request, admin,
        action="blacklist.bulk_import", entity_type="blacklist_entry",
        after={
            "imported": result.imported,
            "skipped": result.skipped,
            "errors": len(result.errors),
        },
    )
    await session.commit()
    return result


# ── Policy mode ──────────────────────────────────────────────────────────────
@policy_router.get("", response_model=PolicyOut, summary="Get policy settings")
async def get_policy(session: DBSession, _: CurrentUser) -> PolicyOut:
    policy = await policy_service.get_policy(session)
    await session.commit()
    return PolicyOut(whitelist_mode=policy.whitelist_mode, flag_unsigned=policy.flag_unsigned)


@policy_router.patch("", response_model=PolicyOut, summary="Update policy settings")
async def update_policy(
    payload: PolicyUpdate, session: DBSession, admin: AdminUser, request: Request
) -> PolicyOut:
    policy = await policy_service.update_policy(
        session,
        whitelist_mode=payload.whitelist_mode,
        flag_unsigned=payload.flag_unsigned,
    )
    await audit(
        session, request, admin,
        action="policy.update", entity_type="policy",
        after={"whitelist_mode": policy.whitelist_mode, "flag_unsigned": policy.flag_unsigned},
    )
    await session.commit()
    return PolicyOut(whitelist_mode=policy.whitelist_mode, flag_unsigned=policy.flag_unsigned)
