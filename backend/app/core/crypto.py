"""Column-level secret encryption (Module 6 — license keys).

Uses Fernet (AES-128-CBC + HMAC) with a key derived from the configured
`license_key_encryption_key` (falling back to `jwt_secret` in dev/test where the
dedicated key is unset). Deriving via SHA-256 → urlsafe-base64 means any secret
string is accepted as input, sidestepping Fernet's strict 32-byte-base64 format.
"""

from __future__ import annotations

import base64
import hashlib
from functools import lru_cache
from typing import Any

from cryptography.fernet import Fernet, InvalidToken
from sqlalchemy import LargeBinary
from sqlalchemy.engine.interfaces import Dialect
from sqlalchemy.types import TypeDecorator

from app.core.config import settings


def _derive_key(secret: str) -> bytes:
    digest = hashlib.sha256(secret.encode("utf-8")).digest()
    return base64.urlsafe_b64encode(digest)


@lru_cache(maxsize=1)
def _fernet() -> Fernet:
    secret = settings.license_key_encryption_key or settings.jwt_secret
    return Fernet(_derive_key(secret))


def encrypt_secret(plain: str | None) -> bytes | None:
    if not plain:
        return None
    return _fernet().encrypt(plain.encode("utf-8"))


def decrypt_secret(cipher: bytes | None) -> str | None:
    if not cipher:
        return None
    try:
        return _fernet().decrypt(bytes(cipher)).decode("utf-8")
    except InvalidToken:
        # Key rotated or corrupt ciphertext — surface as "no key" rather than 500.
        return None


class EncryptedString(TypeDecorator[str]):
    """Transparently AES-encrypts a string at rest. Stored as bytes (ciphertext)."""

    impl = LargeBinary
    cache_ok = True

    def process_bind_param(self, value: str | None, dialect: Dialect) -> bytes | None:
        return encrypt_secret(value)

    def process_result_value(self, value: Any, dialect: Dialect) -> str | None:
        return decrypt_secret(value)
