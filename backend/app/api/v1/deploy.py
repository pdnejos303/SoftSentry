"""One-click deployment endpoints.

Admin endpoints mint/list/revoke reusable *deployment tokens*. The public
``/installer`` endpoint streams a self-installing agent download whose config
trailer carries the server URL + token, so an end user just double-clicks the
file (one-click installer design).
"""

from __future__ import annotations

import uuid as uuid_lib
from typing import Annotated

from fastapi import APIRouter, Depends, HTTPException, Query, Request, Response, status

from app.api.v1._audit import audit
from app.core.config import settings
from app.core.deps import DBSession, require_role
from app.models.user import User
from app.schemas.deploy import (
    BinaryInfoOut,
    DeploymentTokenCreated,
    DeploymentTokenCreateRequest,
    DeploymentTokenOut,
)
from app.services import binary_service, enrollment_service, installer_service

router = APIRouter()

# The acting admin (kept for audit attribution).
AdminUser = Annotated[User, Depends(require_role("dev", "admin"))]


@router.post(
    "/tokens",
    response_model=DeploymentTokenCreated,
    status_code=status.HTTP_201_CREATED,
    summary="Admin: create a reusable deployment token",
)
async def create_deployment_token(
    payload: DeploymentTokenCreateRequest,
    session: DBSession,
    admin: AdminUser,
    request: Request,
) -> DeploymentTokenCreated:
    token, plaintext = await enrollment_service.create_deployment_token(
        session=session,
        label=payload.label,
        expires_in_days=payload.expires_in_days,
        max_uses=payload.max_uses,
        created_by_user_id=admin.id,
    )
    await audit(
        session,
        request,
        admin,
        action="deployment_token.create",
        entity_type="deployment_token",
        entity_id=str(token.uuid),
        after={"label": token.label, "max_uses": token.max_uses},
    )
    await session.commit()
    return DeploymentTokenCreated(
        uuid=token.uuid,
        token=plaintext,
        label=token.label,
        expires_at=token.expires_at,
        max_uses=token.max_uses,
    )


@router.get(
    "/tokens",
    response_model=list[DeploymentTokenOut],
    summary="Admin: list deployment tokens",
)
async def list_deployment_tokens(
    session: DBSession,
    _: AdminUser,
    include_revoked: Annotated[bool, Query()] = False,
) -> list[DeploymentTokenOut]:
    tokens = await enrollment_service.list_tokens(
        session=session, include_revoked=include_revoked
    )
    return [DeploymentTokenOut.model_validate(t) for t in tokens]


@router.post(
    "/tokens/{token_uuid}/revoke",
    response_model=DeploymentTokenOut,
    summary="Admin: revoke a deployment token",
)
async def revoke_deployment_token(
    token_uuid: uuid_lib.UUID,
    session: DBSession,
    admin: AdminUser,
    request: Request,
) -> DeploymentTokenOut:
    try:
        token = await enrollment_service.revoke_token(session=session, token_uuid=token_uuid)
    except enrollment_service.TokenNotFound as exc:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "Token not found") from exc

    await audit(
        session,
        request,
        admin,
        action="deployment_token.revoke",
        entity_type="deployment_token",
        entity_id=str(token.uuid),
    )
    await session.commit()
    return DeploymentTokenOut.model_validate(token)


@router.get(
    "/binary-info",
    response_model=BinaryInfoOut | None,
    summary="Admin: the latest agent binary being served (for the deploy UI version badge)",
)
async def latest_binary_info(
    _: AdminUser,
    os: Annotated[str, Query()] = "windows",
    arch: Annotated[str, Query()] = "amd64",
) -> BinaryInfoOut | None:
    # Mirrors what /installer would hand out: the highest-versioned binary in
    # agent_binary_dir for this os/arch. None (→ null body) when none is published
    # yet, so the badge simply hides instead of erroring.
    entry = binary_service.latest_for(settings.agent_binary_dir, os, arch)
    if entry is None:
        return None
    return BinaryInfoOut(
        version=entry.version,
        os=entry.os,
        arch=entry.arch,
        sha256=entry.sha256,
        build_stamp=entry.build_stamp,
    )


@router.get(
    "/installer",
    summary="Download a self-installing agent (token in query authorizes)",
    responses={
        403: {"description": "Token invalid, revoked, expired, or out of uses"},
        503: {"description": "No agent binary available for this OS/arch"},
    },
)
async def download_installer(
    session: DBSession,
    token: Annotated[str, Query(description="deployment token plaintext")],
    request: Request,
    os: Annotated[str, Query()] = "windows",
    arch: Annotated[str, Query()] = "amd64",
    server: Annotated[str | None, Query(description="callback URL to embed")] = None,
) -> Response:
    # No admin auth here — this link is opened by the end user's browser. The
    # token itself authorizes; we only confirm it is still live so we never hand
    # out installers for revoked/expired tokens.
    valid = await enrollment_service.find_valid_token(session=session, plaintext=token)
    if valid is None:
        raise HTTPException(status.HTTP_403_FORBIDDEN, "Invalid or expired token")

    entry = binary_service.latest_for(settings.agent_binary_dir, os, arch)
    path = (
        binary_service.resolve_path(settings.agent_binary_dir, entry.version, os, arch)
        if entry
        else None
    )
    if path is None:
        raise HTTPException(
            status.HTTP_503_SERVICE_UNAVAILABLE,
            f"No agent binary available for {os}/{arch}. Upload one to agent_binary_dir.",
        )

    server_url = (server or str(request.base_url)).rstrip("/")
    blob = installer_service.build_installer(
        binary=path.read_bytes(),
        server_url=server_url,
        token=token,
    )
    filename = "SoftSentry-Setup.exe" if os == "windows" else f"SoftSentry-Setup-{os}"
    return Response(
        content=blob,
        media_type="application/octet-stream",
        headers={
            "Content-Disposition": f'attachment; filename="{filename}"',
            # installer ต้องสดเสมอ — URL คงที่ต่อ token ทำให้ browser cache bytes เก่า
            # (ผู้ใช้กด download ซ้ำได้ตัวเดิมแม้ volume อัปเดตแล้ว) → ห้าม cache เด็ดขาด
            "Cache-Control": "no-store, no-cache, must-revalidate, max-age=0",
            "Pragma": "no-cache",
            "Expires": "0",
        },
    )
