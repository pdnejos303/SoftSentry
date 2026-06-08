"""Pydantic schemas for auth endpoints."""

from __future__ import annotations

import uuid as uuid_lib
from datetime import datetime
from typing import Literal

from pydantic import BaseModel, ConfigDict, Field, SecretStr, field_validator


class LoginRequest(BaseModel):
    # Plain str (not EmailStr): the login email is a credential lookup key, and the
    # documented default account `admin@local` uses a dotless domain that strict
    # RFC email validation rejects. Format is enforced at user-creation time instead.
    email: str
    password: SecretStr
    remember_me: bool = False

    @field_validator("email")
    @classmethod
    def _normalize_email(cls, v: str) -> str:
        return v.strip().lower()


class TokenPair(BaseModel):
    access_token: str
    token_type: Literal["bearer"] = "bearer"
    expires_in: int = Field(..., description="access token TTL in seconds")


class RefreshResponse(TokenPair):
    pass


class ChangePasswordRequest(BaseModel):
    current_password: SecretStr
    new_password: SecretStr


class UserOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    uuid: uuid_lib.UUID
    email: str
    full_name: str
    role: Literal["dev", "admin", "viewer"]
    is_active: bool
    last_login_at: datetime | None
    created_at: datetime
