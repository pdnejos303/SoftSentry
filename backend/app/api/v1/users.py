"""User management endpoints (Module 9 — admin only). All mutations audited."""

from __future__ import annotations

import uuid as uuid_lib
from typing import Annotated

from fastapi import APIRouter, Depends, HTTPException, Query, Request, status

from app.core.deps import DBSession, require_role
from app.core.password_policy import PasswordPolicyError
from app.models.user import User
from app.schemas.users import (
    PasswordResetResult,
    UserCreate,
    UserCreated,
    UserItem,
    UserList,
    UserUpdate,
)
from app.services import audit_service, user_service

router = APIRouter()

# The acting admin (not discarded — needed for audit attribution).
AdminUser = Annotated[User, Depends(require_role("dev"))]


def _pages(total: int, page_size: int) -> int:
    return (total + page_size - 1) // page_size


def _ip(request: Request) -> str | None:
    fwd = request.headers.get("x-forwarded-for")
    if fwd:
        return fwd.split(",")[0].strip()
    return request.client.host if request.client else None


@router.get("", response_model=UserList, summary="List dashboard users")
async def list_users(
    session: DBSession,
    _: AdminUser,
    q: str | None = None,
    role: str | None = None,
    is_active: bool | None = None,
    page: Annotated[int, Query(ge=1)] = 1,
    page_size: Annotated[int, Query(ge=1, le=200)] = 50,
) -> UserList:
    users, total = await user_service.list_users(
        session, q=q, role=role, is_active=is_active, page=page, page_size=page_size
    )
    return UserList(
        items=[UserItem.model_validate(u) for u in users],
        total=total,
        page=page,
        page_size=page_size,
        total_pages=_pages(total, page_size),
    )


@router.post(
    "", response_model=UserCreated, status_code=status.HTTP_201_CREATED, summary="Create user"
)
async def create_user(
    payload: UserCreate,
    session: DBSession,
    admin: AdminUser,
    request: Request,
) -> UserCreated:
    try:
        user, plaintext = await user_service.create_user(
            session,
            email=payload.email,
            full_name=payload.full_name,
            role=payload.role,
            password=payload.password.get_secret_value() if payload.password else None,
        )
    except user_service.EmailAlreadyExists as exc:
        raise HTTPException(status.HTTP_409_CONFLICT, "Email already in use") from exc
    except PasswordPolicyError as exc:
        raise HTTPException(status.HTTP_422_UNPROCESSABLE_ENTITY, str(exc)) from exc

    await audit_service.record(
        session,
        user_id=admin.id,
        action="user.create",
        entity_type="user",
        entity_id=str(user.uuid),
        after={"email": user.email, "role": user.role, "full_name": user.full_name},
        ip=_ip(request),
        user_agent=request.headers.get("user-agent"),
    )
    await session.commit()
    return UserCreated(**UserItem.model_validate(user).model_dump(), initial_password=plaintext)


@router.get("/{user_uuid}", response_model=UserItem, summary="Get user")
async def get_user(user_uuid: uuid_lib.UUID, session: DBSession, _: AdminUser) -> UserItem:
    user = await user_service.get_by_uuid(session, user_uuid)
    if user is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "User not found")
    return UserItem.model_validate(user)


@router.patch("/{user_uuid}", response_model=UserItem, summary="Update user")
async def update_user(
    user_uuid: uuid_lib.UUID,
    payload: UserUpdate,
    session: DBSession,
    admin: AdminUser,
    request: Request,
) -> UserItem:
    user = await user_service.get_by_uuid(session, user_uuid)
    if user is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "User not found")

    # Guard: an admin demoting/disabling themselves while last admin.
    try:
        changes = await user_service.update_user(
            session,
            user=user,
            full_name=payload.full_name,
            role=payload.role,
            is_active=payload.is_active,
        )
    except user_service.LastAdminProtected as exc:
        raise HTTPException(
            status.HTTP_409_CONFLICT, "Cannot demote or disable the last remaining admin"
        ) from exc

    if changes:
        await audit_service.record(
            session,
            user_id=admin.id,
            action="user.update",
            entity_type="user",
            entity_id=str(user.uuid),
            before={k: v["before"] for k, v in changes.items()},
            after={k: v["after"] for k, v in changes.items()},
            ip=_ip(request),
            user_agent=request.headers.get("user-agent"),
        )
    await session.commit()
    await session.refresh(user)
    return UserItem.model_validate(user)


@router.delete("/{user_uuid}", status_code=status.HTTP_204_NO_CONTENT, summary="Delete user (soft)")
async def delete_user(
    user_uuid: uuid_lib.UUID,
    session: DBSession,
    admin: AdminUser,
    request: Request,
) -> None:
    user = await user_service.get_by_uuid(session, user_uuid)
    if user is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "User not found")
    try:
        await user_service.soft_delete_user(session, user=user, acting_user_id=admin.id)
    except user_service.CannotDeleteSelf as exc:
        raise HTTPException(status.HTTP_409_CONFLICT, "Cannot delete your own account") from exc
    except user_service.LastAdminProtected as exc:
        raise HTTPException(
            status.HTTP_409_CONFLICT, "Cannot delete the last remaining admin"
        ) from exc

    await audit_service.record(
        session,
        user_id=admin.id,
        action="user.delete",
        entity_type="user",
        entity_id=str(user.uuid),
        before={"email": user.email, "role": user.role},
        ip=_ip(request),
        user_agent=request.headers.get("user-agent"),
    )
    await session.commit()


@router.post(
    "/{user_uuid}/reset-password",
    response_model=PasswordResetResult,
    summary="Reset a user's password (admin)",
)
async def reset_password(
    user_uuid: uuid_lib.UUID,
    session: DBSession,
    admin: AdminUser,
    request: Request,
) -> PasswordResetResult:
    user = await user_service.get_by_uuid(session, user_uuid)
    if user is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, "User not found")
    new_password = await user_service.reset_password(session, user=user)
    await audit_service.record(
        session,
        user_id=admin.id,
        action="user.password_reset",
        entity_type="user",
        entity_id=str(user.uuid),
        ip=_ip(request),
        user_agent=request.headers.get("user-agent"),
    )
    await session.commit()
    return PasswordResetResult(uuid=user.uuid, new_password=new_password)
