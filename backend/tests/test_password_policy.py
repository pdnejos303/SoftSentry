"""Password policy validator unit tests (Module 9 — spec 9.8)."""

from __future__ import annotations

import pytest
from app.core.password_policy import (
    PasswordPolicyError,
    generate_password,
    validate_password,
)


def test_accepts_strong_password() -> None:
    # No exception == accepted.
    validate_password("Str0ng!Passw0rd")


def test_rejects_too_short() -> None:
    with pytest.raises(PasswordPolicyError) as exc:
        validate_password("Ab1!def")
    assert "12" in str(exc.value)


def test_rejects_missing_uppercase() -> None:
    with pytest.raises(PasswordPolicyError) as exc:
        validate_password("str0ng!passw0rd")
    assert "uppercase" in str(exc.value).lower()


def test_rejects_missing_lowercase() -> None:
    with pytest.raises(PasswordPolicyError) as exc:
        validate_password("STR0NG!PASSW0RD")
    assert "lowercase" in str(exc.value).lower()


def test_rejects_missing_digit() -> None:
    with pytest.raises(PasswordPolicyError) as exc:
        validate_password("Strong!Password")
    assert "digit" in str(exc.value).lower()


def test_rejects_missing_special() -> None:
    with pytest.raises(PasswordPolicyError) as exc:
        validate_password("Str0ngPassw0rd")
    assert "special" in str(exc.value).lower()


def test_rejects_common_password() -> None:
    # Long enough + complex but in the common list.
    with pytest.raises(PasswordPolicyError) as exc:
        validate_password("Password123!")
    assert "common" in str(exc.value).lower()


def test_common_check_is_case_insensitive() -> None:
    with pytest.raises(PasswordPolicyError):
        validate_password("PASSWORD123!")


def test_generated_password_passes_policy() -> None:
    for _ in range(50):
        pw = generate_password()
        validate_password(pw)  # must not raise
        assert len(pw) >= 16
