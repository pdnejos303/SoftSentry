"""Pure unit tests for security primitives."""

from __future__ import annotations

import jwt
import pytest
from app.core.security import (
    create_token,
    decode_token,
    generate_opaque_token,
    hash_password,
    verify_password,
)


def test_hash_password_verifies():
    h = hash_password("S3cret!Pass-Word")
    assert verify_password("S3cret!Pass-Word", h)
    assert not verify_password("S3cret!Pass-Wor", h)
    assert not verify_password("", h)


def test_hash_password_unique_per_call():
    a = hash_password("same")
    b = hash_password("same")
    assert a != b  # bcrypt salt


def test_access_token_roundtrip():
    token, exp = create_token(subject="user-123", token_type="access")
    payload = decode_token(token, expected_type="access")
    assert payload["sub"] == "user-123"
    assert payload["type"] == "access"
    assert exp.timestamp() == payload["exp"]


def test_refresh_token_type_mismatch_rejected():
    token, _ = create_token(subject="u", token_type="refresh")
    with pytest.raises(jwt.InvalidTokenError):
        decode_token(token, expected_type="access")


def test_opaque_token_length_and_charset():
    t = generate_opaque_token(num_bytes=32)
    assert len(t) >= 40
    # base64url charset only
    assert set(t) <= set("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_")


def test_opaque_tokens_are_unique():
    seen = {generate_opaque_token() for _ in range(100)}
    assert len(seen) == 100
