"""Dashboard-facing vulnerability endpoints — list, detail, dismiss, CVE lookup,
fleet summary, and admin manual sync (Module 5)."""

from __future__ import annotations

import uuid as uuid_lib
from typing import Annotated

from fastapi import APIRouter, Depends, HTTPException, Query, Request, status

from app.api.v1._audit import audit
from app.core.deps import CurrentUser, DBSession, require_role
from app.models.cve import Vulnerability
from app.models.user import User
from app.schemas.vulnerability import (
    CveDetail,
    CveSyncResult,
    DismissRequest,
    VulnerabilityDetail,
    VulnerabilityItem,
    VulnerabilityList,
    VulnSummary,
)
from app.services import vulnerability_service
from app.workers.cve_sync import enqueue_cve_sync

router = APIRouter()
cve_router = APIRouter()
dashboard_router = APIRouter()
admin_router = APIRouter()


@router.get("", response_model=VulnerabilityList, summary="List vulnerabilities")
async def list_vulnerabilities(
    session: DBSession,
    _: CurrentUser,
    severity: str | None = None,  # comma-separated
    machine_uuid: uuid_lib.UUID | None = None,
    show_dismissed: bool = False,
    date_window: Annotated[str | None, Query(alias="matched")] = None,  # 7d|30d|90d|all
    q: str | None = None,
    min_confidence: str | None = None,
    page: Annotated[int, Query(ge=1)] = 1,
    page_size: Annotated[int, Query(ge=1, le=200)] = 50,
) -> VulnerabilityList:
    severities = [s.strip() for s in severity.split(",") if s.strip()] if severity else None
    items, total = await vulnerability_service.list_vulnerabilities(
        session,
        severities=severities,
        machine_uuid=machine_uuid,
        show_dismissed=show_dismissed,
        date_window=date_window,
        q=q,
        min_confidence=min_confidence,
        page=page,
        page_size=page_size,
    )
    total_pages = (total + page_size - 1) // page_size
    return VulnerabilityList(
        items=[VulnerabilityItem(**i) for i in items],
        total=total,
        page=page,
        page_size=page_size,
        total_pages=total_pages,
    )


@router.get("/{vuln_uuid}", response_model=VulnerabilityDetail, summary="Vulnerability detail")
async def vulnerability_detail(
    vuln_uuid: uuid_lib.UUID, session: DBSession, _: CurrentUser
) -> VulnerabilityDetail:
    detail = await vulnerability_service.vulnerability_detail(session, vuln_uuid)
    if detail is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "Vulnerability not found")
    return VulnerabilityDetail(**detail)


async def _get_vuln_or_404(session: DBSession, vuln_uuid: uuid_lib.UUID) -> Vulnerability:
    vuln = await vulnerability_service.get_vulnerability(session, vuln_uuid)
    if vuln is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "Vulnerability not found")
    return vuln


@router.post(
    "/{vuln_uuid}/dismiss",
    response_model=VulnerabilityDetail,
    summary="Admin: dismiss a vulnerability (false positive / accepted risk)",
)
async def dismiss_vulnerability(
    vuln_uuid: uuid_lib.UUID,
    payload: DismissRequest,
    session: DBSession,
    user: Annotated[User, Depends(require_role("dev"))],
    request: Request,
) -> VulnerabilityDetail:
    vuln = await _get_vuln_or_404(session, vuln_uuid)
    await vulnerability_service.dismiss(session, vuln, user=user, reason=payload.reason)
    await audit(
        session, request, user,
        action="vulnerability.dismiss", entity_type="vulnerability", entity_id=str(vuln_uuid),
        after={"reason": payload.reason},
    )
    await session.commit()
    detail = await vulnerability_service.vulnerability_detail(session, vuln_uuid)
    assert detail is not None
    return VulnerabilityDetail(**detail)


@router.post(
    "/{vuln_uuid}/undismiss",
    response_model=VulnerabilityDetail,
    summary="Admin: restore a dismissed vulnerability",
)
async def undismiss_vulnerability(
    vuln_uuid: uuid_lib.UUID,
    session: DBSession,
    user: Annotated[User, Depends(require_role("dev"))],
    request: Request,
) -> VulnerabilityDetail:
    vuln = await _get_vuln_or_404(session, vuln_uuid)
    await vulnerability_service.undismiss(session, vuln)
    await audit(
        session, request, user,
        action="vulnerability.undismiss", entity_type="vulnerability", entity_id=str(vuln_uuid),
    )
    await session.commit()
    detail = await vulnerability_service.vulnerability_detail(session, vuln_uuid)
    assert detail is not None
    return VulnerabilityDetail(**detail)


@cve_router.get("/{cve_id}", response_model=CveDetail, summary="Raw CVE record")
async def cve_detail(cve_id: str, session: DBSession, _: CurrentUser) -> CveDetail:
    cve = await vulnerability_service.get_cve(session, cve_id)
    if cve is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "CVE not found")
    return CveDetail(
        cve_id=cve.cve_id,
        source=cve.source,
        severity=cve.severity,
        cvss_score=float(cve.cvss_score) if cve.cvss_score is not None else None,
        description=cve.description,
        published_at=cve.published_at,
        modified_at=cve.modified_at,
        references=cve.references or [],
        affected=cve.affected,
    )


@dashboard_router.get("/vuln-summary", response_model=VulnSummary, summary="Severity distribution")
async def vuln_summary(session: DBSession, _: CurrentUser) -> VulnSummary:
    return VulnSummary(**await vulnerability_service.summary(session))


@admin_router.post(
    "/cve-sync/trigger",
    response_model=CveSyncResult,
    status_code=status.HTTP_202_ACCEPTED,
    summary="Admin: trigger a CVE sync now",
)
async def trigger_cve_sync(
    session: DBSession,
    user: Annotated[User, Depends(require_role("dev"))],
    request: Request,
    full: bool = False,
) -> CveSyncResult:
    queued = await enqueue_cve_sync(full=full)
    if not queued:
        raise HTTPException(status.HTTP_503_SERVICE_UNAVAILABLE, "Job queue unavailable")
    await audit(
        session, request, user,
        action="cve_sync.trigger", entity_type="cve_sync", after={"full": full},
    )
    await session.commit()
    # The job runs asynchronously; counts are reported via worker logs/metrics.
    return CveSyncResult(inserted=0, updated=0, skipped=0, total=0)
