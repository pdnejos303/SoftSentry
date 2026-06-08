"""Pydantic schemas for user management + audit log (Module 9)."""

from __future__ import annotations

import uuid as uuid_lib
from datetime import datetime
from typing import Any, Literal

from pydantic import BaseModel, ConfigDict, EmailStr, Field, SecretStr

Role = Literal["dev", "admin", "viewer"]


class UserItem(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    uuid: uuid_lib.UUID
    email: str
    full_name: str
    role: Role
    is_active: bool
    last_login_at: datetime | None
    created_at: datetime


class UserList(BaseModel):
    items: list[UserItem]
    total: int
    page: int
    page_size: int
    total_pages: int


class UserCreate(BaseModel):
    email: EmailStr
    full_name: str = Field(min_length=1, max_length=255)
    role: Role = "viewer"
    # Optional: when omitted the backend generates a strong random password.
    password: SecretStr | None = None


class UserUpdate(BaseModel):
    full_name: str | None = Field(default=None, min_length=1, max_length=255)
    role: Role | None = None
    is_active: bool | None = None


class UserCreated(UserItem):
    # Shown exactly once to the admin (create / reset-password).
    initial_password: str


class PasswordResetResult(BaseModel):
    uuid: uuid_lib.UUID
    new_password: str


class AuditLogItem(BaseModel):
    id: int
    actor_uuid: uuid_lib.UUID | None
    actor_email: str | None
    actor_name: str | None
    action: str
    entity_type: str
    entity_id: str | None
    changes: dict[str, Any] | None
    ip_address: str | None
    user_agent: str | None
    created_at: datetime


class AuditLogList(BaseModel):
    items: list[AuditLogItem]
    total: int
    page: int
    page_size: int
    total_pages: int
