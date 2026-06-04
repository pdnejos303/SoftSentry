"""Dashboard user authentication endpoints."""

from __future__ import annotations

from fastapi import APIRouter, Cookie, HTTPException, Request, Response, status

from app.core.config import settings
from app.core.deps import CurrentUser, DBSession, RedisDep, client_ip
from app.schemas.auth import (
    ChangePasswordRequest,
    LoginRequest,
    RefreshResponse,
    TokenPair,
    UserOut,
)
from app.services import audit_service, auth_service

router = APIRouter()

REFRESH_COOKIE = "softsentry_refresh"


def _set_refresh_cookie(response: Response, value: str, max_age: int) -> None:
    response.set_cookie(
        REFRESH_COOKIE,
        value,
        max_age=max_age,
        httponly=True,
        secure=settings.is_production,
        samesite="strict",
        path="/api/v1/auth",
    )


def _clear_refresh_cookie(response: Response) -> None:
    response.delete_cookie(REFRESH_COOKIE, path="/api/v1/auth")


@router.post("/login", response_model=TokenPair)
async def login(
    payload: LoginRequest,
    request: Request,
    response: Response,
    session: DBSession,
    redis: RedisDep,
) -> TokenPair:
    ip = client_ip(request)
    ua = request.headers.get("user-agent")
    try:
        user = await auth_service.authenticate(
            session=session,
            redis=redis,
            email=payload.email,
            password=payload.password.get_secret_value(),
            ip=ip,
        )
    except auth_service.AccountLocked as exc:
        raise HTTPException(
            status.HTTP_429_TOO_MANY_REQUESTS,
            "Account temporarily locked",
            headers={"Retry-After": str(exc.retry_after_seconds)},
        ) from exc
    except (auth_service.AccountInactive, auth_service.InvalidCredentials) as exc:
        await audit_service.record(
            session,
            action="auth.login_failure",
            entity_type="auth",
            entity_id=payload.email,
            ip=ip,
            user_agent=ua,
        )
        await session.commit()
        if isinstance(exc, auth_service.AccountInactive):
            raise HTTPException(status.HTTP_403_FORBIDDEN, "Account is disabled") from exc
        raise HTTPException(status.HTTP_401_UNAUTHORIZED, "Incorrect email or password") from exc

    access, refresh, ttl = await auth_service.issue_token_pair(
        redis=redis, user=user, remember_me=payload.remember_me
    )
    max_age = settings.jwt_refresh_ttl_seconds if payload.remember_me else 24 * 3600
    _set_refresh_cookie(response, refresh, max_age)
    await audit_service.record(
        session,
        user_id=user.id,
        action="auth.login_success",
        entity_type="auth",
        entity_id=str(user.uuid),
        ip=ip,
        user_agent=ua,
    )
    await session.commit()
    return TokenPair(access_token=access, expires_in=ttl)


@router.post("/refresh", response_model=RefreshResponse)
async def refresh_tokens(
    response: Response,
    session: DBSession,
    redis: RedisDep,
    softsentry_refresh: str | None = Cookie(default=None),
) -> RefreshResponse:
    if not softsentry_refresh:
        raise HTTPException(status.HTTP_401_UNAUTHORIZED, "Missing refresh cookie")
    try:
        access, new_refresh, ttl = await auth_service.rotate_refresh(
            session=session, redis=redis, refresh_token=softsentry_refresh
        )
    except auth_service.AuthError as exc:
        _clear_refresh_cookie(response)
        raise HTTPException(status.HTTP_401_UNAUTHORIZED, "Invalid refresh token") from exc

    _set_refresh_cookie(response, new_refresh, settings.jwt_refresh_ttl_seconds)
    return RefreshResponse(access_token=access, expires_in=ttl)


@router.post("/logout", status_code=status.HTTP_204_NO_CONTENT)
async def logout(
    response: Response,
    redis: RedisDep,
    softsentry_refresh: str | None = Cookie(default=None),
) -> Response:
    if softsentry_refresh:
        await auth_service.revoke_refresh(redis=redis, refresh_token=softsentry_refresh)
    _clear_refresh_cookie(response)
    return Response(status_code=status.HTTP_204_NO_CONTENT)


@router.get("/me", response_model=UserOut)
async def me(current: CurrentUser) -> UserOut:
    return UserOut.model_validate(current)


@router.post("/change-password", status_code=status.HTTP_204_NO_CONTENT)
async def change_password(
    payload: ChangePasswordRequest,
    current: CurrentUser,
    session: DBSession,
) -> Response:
    try:
        await auth_service.change_password(
            session=session,
            user=current,
            current_password=payload.current_password.get_secret_value(),
            new_password=payload.new_password.get_secret_value(),
        )
    except auth_service.InvalidCredentials as exc:
        raise HTTPException(status.HTTP_400_BAD_REQUEST, "Current password is incorrect") from exc
    await session.commit()
    return Response(status_code=status.HTTP_204_NO_CONTENT)
