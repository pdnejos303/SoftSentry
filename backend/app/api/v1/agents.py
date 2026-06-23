"""Agent-facing endpoints — enrollment + (future) heartbeat/scans."""

from __future__ import annotations

from datetime import UTC, datetime
from typing import Annotated

from fastapi import APIRouter, Depends, Header, HTTPException, status
from fastapi.responses import FileResponse, Response

from app.core import metrics
from app.core.config import settings
from app.core.deps import CurrentAgent, DBSession, require_role
from app.models.agent_config import AgentConfig
from app.models.machine import Machine
from app.schemas.agents import (
    AgentConfigOut,
    AgentEnrollRequest,
    AgentEnrollResponse,
    AgentUpdateAvailable,
    EnrollmentTokenCreated,
    EnrollmentTokenCreateRequest,
    HeartbeatRequest,
    HeartbeatResponse,
    MachineOut,
    ScanProgressIn,
)
from app.schemas.scans import ScanAccepted, ScanIn
from app.services import (
    binary_service,
    enrollment_service,
    risk_service,
    scan_service,
    signature_service,
    unauthorized_service,
    vulnerability_service,
)

router = APIRouter()


@router.post(
    "/enrollment-tokens",
    response_model=EnrollmentTokenCreated,
    status_code=status.HTTP_201_CREATED,
    summary="Admin: generate one-time enrollment token",
    dependencies=[Depends(require_role("dev", "admin"))],
)
async def create_enrollment_token(
    payload: EnrollmentTokenCreateRequest,
    session: DBSession,
) -> EnrollmentTokenCreated:
    token, plaintext = await enrollment_service.create_enrollment_token(
        session=session,
        label=payload.label,
        expires_in_seconds=payload.expires_in_seconds,
        created_by_user_id=None,
    )
    await session.commit()
    return EnrollmentTokenCreated(
        uuid=token.uuid,
        token=plaintext,
        expires_at=token.expires_at,
        label=token.label,
    )


@router.post(
    "/enroll",
    response_model=AgentEnrollResponse,
    status_code=status.HTTP_201_CREATED,
    summary="Agent: exchange one-time enrollment token for permanent agent token",
)
async def enroll(
    payload: AgentEnrollRequest,
    session: DBSession,
) -> AgentEnrollResponse:
    try:
        machine, agent_token = await enrollment_service.enroll_agent(
            session=session,
            enrollment_token_plaintext=payload.enrollment_token,
            hostname=payload.hostname,
            os=payload.os,
            os_version=payload.os_version,
            arch=payload.arch,
            agent_version=payload.agent_version,
            fingerprint=payload.fingerprint,
        )
    except enrollment_service.InvalidEnrollmentToken as exc:
        raise HTTPException(
            status.HTTP_401_UNAUTHORIZED,
            "Invalid or expired enrollment token",
        ) from exc

    await session.commit()
    return AgentEnrollResponse(
        machine_uuid=machine.uuid,
        agent_token=agent_token,
        scan_interval_hours=6,
    )


@router.post(
    "/heartbeat",
    response_model=HeartbeatResponse,
    summary="Agent: liveness ping (default every 60s)",
)
async def heartbeat(
    agent: CurrentAgent,
    session: DBSession,
    payload: HeartbeatRequest | None = None,
) -> HeartbeatResponse:
    agent.last_seen_at = datetime.now(tz=UTC)
    agent.status = "online"
    if payload and payload.agent_version:
        agent.agent_version = payload.agent_version
    if payload and payload.server_url:
        agent.reported_server_url = payload.server_url
    _apply_scan_progress(agent, payload.scan_progress if payload else None)

    cfg = await session.get(AgentConfig, agent.id)

    # Forced update wins: an admin pressed "Update agent now". Offer the latest
    # binary regardless of version/auto-update, then clear the flag in this same
    # committed transaction so it fires exactly once (no restart loop).
    update_offer: AgentUpdateAvailable | None = None
    if cfg and cfg.force_update_requested:
        update_offer = _forced_update(agent.os, agent.arch)
        cfg.force_update_requested = False
    elif (cfg.auto_update_enabled if cfg else True):
        update_offer = _offer_update(agent.os, agent.arch, agent.agent_version)

    await session.commit()

    return HeartbeatResponse(
        config_changed=False,
        manual_scan_requested=bool(cfg and cfg.manual_scan_requested),
        agent_update_available=update_offer,
    )


def _apply_scan_progress(agent: Machine, progress: ScanProgressIn | None) -> None:
    """Store live scan progress from a heartbeat, or reset to idle when absent.

    The agent only sends ``scan_progress`` mid-scan; an idle heartbeat omits it,
    which we treat as "no scan running" and clear so the dashboard hides the bar.
    """
    if progress is None or progress.phase in ("", "idle"):
        agent.scan_phase = "idle"
        agent.scan_done = None
        agent.scan_total = None
        agent.scan_current_path = None
        agent.scan_progress_at = progress.updated_at if progress else datetime.now(tz=UTC)
        return
    agent.scan_phase = progress.phase
    agent.scan_done = progress.done
    agent.scan_total = progress.total
    agent.scan_current_path = progress.current_path
    agent.scan_progress_at = progress.updated_at or datetime.now(tz=UTC)


def _offer_update(os: str, arch: str, current_version: str) -> AgentUpdateAvailable | None:
    """Return an update offer iff a strictly newer binary exists for this OS/arch."""
    latest = binary_service.latest_for(settings.agent_binary_dir, os, arch)
    if latest is None or not binary_service.is_newer(latest.version, current_version):
        return None
    return AgentUpdateAvailable(
        version=latest.version,
        download_url=binary_service.download_url(latest.version, os, arch),
        sha256=latest.sha256,
    )


def _forced_update(os: str, arch: str) -> AgentUpdateAvailable | None:
    """Return the latest binary as a *forced* offer (admin "Update agent now").

    Ignores version comparison and auto-update entirely — the agent will apply it
    even if it matches the running version (reinstall/repair). Still None when no
    binary exists for this platform, but the trigger-update endpoint already
    rejects that case with 409 so it is a defensive guard here.
    """
    latest = binary_service.latest_for(settings.agent_binary_dir, os, arch)
    if latest is None:
        return None
    return AgentUpdateAvailable(
        version=latest.version,
        download_url=binary_service.download_url(latest.version, os, arch),
        sha256=latest.sha256,
        forced=True,
    )


@router.get(
    "/binary/latest",
    response_model=AgentUpdateAvailable,
    summary="Agent: get the latest available binary for this OS/arch",
    responses={204: {"description": "No binary available for this OS/arch"}},
)
async def latest_binary(agent: CurrentAgent) -> AgentUpdateAvailable | Response:
    latest = binary_service.latest_for(settings.agent_binary_dir, agent.os, agent.arch)
    if latest is None:
        return Response(status_code=status.HTTP_204_NO_CONTENT)
    return AgentUpdateAvailable(
        version=latest.version,
        download_url=binary_service.download_url(latest.version, agent.os, agent.arch),
        sha256=latest.sha256,
    )


@router.get(
    "/binary/download/{version}/{os}/{arch}",
    summary="Agent: download a specific binary",
    response_class=FileResponse,
)
async def download_binary(
    agent: CurrentAgent,
    version: str,
    os: str,
    arch: str,
) -> FileResponse:
    path = binary_service.resolve_path(settings.agent_binary_dir, version, os, arch)
    if path is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "Binary not found")
    return FileResponse(
        path,
        media_type="application/octet-stream",
        filename=path.name,
    )


@router.get(
    "/config",
    response_model=AgentConfigOut,
    summary="Agent: pull current configuration",
)
async def get_config(agent: CurrentAgent, session: DBSession) -> AgentConfigOut:
    cfg = await session.get(AgentConfig, agent.id)
    if cfg is None:
        return AgentConfigOut()
    return AgentConfigOut(
        scan_interval_hours=cfg.scan_interval_hours,
        auto_update_enabled=cfg.auto_update_enabled,
        manual_scan_requested=cfg.manual_scan_requested,
    )


@router.post(
    "/scans",
    response_model=ScanAccepted,
    status_code=status.HTTP_202_ACCEPTED,
    summary="Agent: submit a software inventory scan",
)
async def submit_scan(
    agent: CurrentAgent,
    session: DBSession,
    payload: ScanIn,
    idempotency_key: Annotated[str | None, Header(alias="Idempotency-Key")] = None,
) -> ScanAccepted:
    try:
        scan = await scan_service.process_scan(
            session=session,
            machine=agent,
            payload=payload,
            idempotency_key=idempotency_key,
        )
        # Detection engines run inline within the scan transaction so alerts
        # appear within one scan cycle (spec 4.4 / 3.4 / 5.7).
        await unauthorized_service.evaluate_machine(session, agent)
        await signature_service.evaluate_machine(session, agent)
        await vulnerability_service.evaluate_machine(session, agent)
        # Risk score folds the above signals into a single stored number
        # (Module 7), so it must run last in the same transaction.
        await risk_service.recompute_machine(session, agent)
        await session.commit()
    except Exception:
        metrics.agent_scan_errors_total.inc()
        raise

    # Telemetry: agents push scan timing/size via the API; the backend turns it
    # into metrics (agents are never scraped directly).
    metrics.observe_scan(
        scan_type=payload.scan_type,
        duration_seconds=(payload.completed_at - payload.started_at).total_seconds(),
        software_count=len(payload.software),
    )
    return ScanAccepted(scan_uuid=scan.uuid, queued_for_analysis=True)


@router.get(
    "/me",
    response_model=MachineOut,
    summary="Agent: self-describe (debugging)",
)
async def agent_me(agent: CurrentAgent) -> MachineOut:
    return MachineOut.model_validate(agent)
