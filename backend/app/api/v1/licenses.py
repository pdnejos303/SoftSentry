"""License compliance endpoints — CRUD, install counts, drill-down, summary,
and a manual refresh that reconciles license alerts (Module 6)."""

from __future__ import annotations

import uuid as uuid_lib
from typing import Annotated

from fastapi import APIRouter, Depends, HTTPException, Query, Request, status

from app.api.v1._audit import audit
from app.core.deps import CurrentUser, DBSession, require_role
from app.models.user import User
from app.schemas.license import (
    ComplianceSummary,
    LicenseDetail,
    LicenseIn,
    LicenseInstallation,
    LicenseInstallationList,
    LicenseList,
    LicenseOut,
    LicenseUpdate,
    RefreshResult,
)
from app.services import license_service

# The acting admin (not discarded — needed for audit attribution).
AdminUser = Annotated[User, Depends(require_role("dev"))]

# license_key is a secret (encrypted at rest) — never copy it into an audit row.
_AUDIT_FIELDS = ("software_name", "vendor", "purchased_count", "seat_type", "expiry_date")

router = APIRouter()


def _pages(total: int, page_size: int) -> int:
    return (total + page_size - 1) // page_size


def _license_snapshot(lic: object) -> dict[str, object]:
    """Non-secret license fields for audit before/after, stringified for JSON."""
    snap: dict[str, object] = {}
    for f in _AUDIT_FIELDS:
        val = getattr(lic, f, None)
        snap[f] = str(val) if val is not None else None
    return snap


async def _detail_response(session: DBSession, license_uuid: uuid_lib.UUID) -> LicenseDetail:
    """Serialize from a fresh, clean fetch — mutations leave the ORM object dirty
    (and autoflush expires it via `updated_at` onupdate), so we re-SELECT."""
    lic = await license_service.get_license(session, license_uuid)
    if lic is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "License not found")
    return LicenseDetail(**await license_service.detail(session, lic))


@router.get("", response_model=LicenseList, summary="List licenses")
async def list_licenses(
    session: DBSession,
    _: CurrentUser,
    q: str | None = None,
    status_: Annotated[str | None, Query(alias="status")] = None,
    page: Annotated[int, Query(ge=1)] = 1,
    page_size: Annotated[int, Query(ge=1, le=200)] = 50,
) -> LicenseList:
    items, total = await license_service.list_licenses(
        session, q=q, status_filter=status_, page=page, page_size=page_size
    )
    return LicenseList(
        items=[LicenseOut(**i) for i in items],
        total=total,
        page=page,
        page_size=page_size,
        total_pages=_pages(total, page_size),
    )


@router.get("/compliance-summary", response_model=ComplianceSummary, summary="Compliance summary")
async def compliance_summary(session: DBSession, _: CurrentUser) -> ComplianceSummary:
    return ComplianceSummary(**await license_service.compliance_summary(session))


@router.post(
    "/refresh-counts",
    response_model=RefreshResult,
    summary="Admin: recompute counts + reconcile license alerts now",
    dependencies=[Depends(require_role("dev"))],
)
async def refresh_counts(session: DBSession, admin: AdminUser, request: Request) -> RefreshResult:
    result = await license_service.reconcile_alerts(session)
    await audit(
        session, request, admin,
        action="license.refresh_counts", entity_type="license",
        after={k: result.get(k) for k in ("licenses", "alerts_raised", "alerts_resolved")},
    )
    await session.commit()
    return RefreshResult(**result)


@router.post(
    "",
    response_model=LicenseDetail,
    status_code=status.HTTP_201_CREATED,
    summary="Add license",
)
async def create_license(
    payload: LicenseIn, session: DBSession, admin: AdminUser, request: Request
) -> LicenseDetail:
    lic = await license_service.create_license(session, payload.model_dump())
    new_uuid = lic.uuid  # client-side default, set at flush
    await audit(
        session, request, admin,
        action="license.create", entity_type="license", entity_id=str(new_uuid),
        after=_license_snapshot(lic),
    )
    await session.commit()
    return await _detail_response(session, new_uuid)


@router.get("/{license_uuid}", response_model=LicenseDetail, summary="License detail")
async def get_license(
    license_uuid: uuid_lib.UUID, session: DBSession, _: CurrentUser
) -> LicenseDetail:
    lic = await license_service.get_license(session, license_uuid)
    if lic is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "License not found")
    return LicenseDetail(**await license_service.detail(session, lic))


@router.patch("/{license_uuid}", response_model=LicenseDetail, summary="Edit license")
async def update_license(
    license_uuid: uuid_lib.UUID,
    payload: LicenseUpdate,
    session: DBSession,
    admin: AdminUser,
    request: Request,
) -> LicenseDetail:
    lic = await license_service.get_license(session, license_uuid)
    if lic is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "License not found")
    before = _license_snapshot(lic)
    license_service.update_license(lic, payload.model_dump(exclude_unset=True))
    await audit(
        session, request, admin,
        action="license.update", entity_type="license", entity_id=str(license_uuid),
        before=before, after=_license_snapshot(lic),
    )
    await session.commit()
    return await _detail_response(session, license_uuid)


@router.delete(
    "/{license_uuid}", status_code=status.HTTP_204_NO_CONTENT, summary="Delete license"
)
async def delete_license(
    license_uuid: uuid_lib.UUID, session: DBSession, admin: AdminUser, request: Request
) -> None:
    lic = await license_service.get_license(session, license_uuid)
    if lic is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "License not found")
    before = _license_snapshot(lic)
    await license_service.delete_license(session, lic)
    await audit(
        session, request, admin,
        action="license.delete", entity_type="license", entity_id=str(license_uuid),
        before=before,
    )
    await session.commit()


@router.get(
    "/{license_uuid}/installations",
    response_model=LicenseInstallationList,
    summary="Machines that have this license's software installed",
)
async def license_installations(
    license_uuid: uuid_lib.UUID, session: DBSession, _: CurrentUser
) -> LicenseInstallationList:
    lic = await license_service.get_license(session, license_uuid)
    if lic is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "License not found")
    rows = await license_service.installations(session, lic)
    return LicenseInstallationList(
        license_uuid=lic.uuid,
        software_name=lic.software_name,
        items=[LicenseInstallation(**r) for r in rows],
        total=len(rows),
    )
