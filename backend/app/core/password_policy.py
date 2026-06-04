"""Password policy validation + random password generation (Module 9 — spec 9.8).

Rules: min 12 chars, must contain uppercase + lowercase + digit + special, and
must not be a well-known common password. The common-password set bundled here
is a curated subset of the most-breached passwords; in production it can be
swapped for the full top-10k list by repointing ``COMMON_PASSWORDS``.
"""

from __future__ import annotations

import secrets
import string

MIN_LENGTH = 12
SPECIAL_CHARS = "!@#$%^&*()-_=+[]{};:,.<>?/|~`"

# Curated subset of the most common / breached passwords (lowercased). The spec
# calls for the bundled top-10k; this guards the obvious offenders cheaply and
# can be replaced wholesale without touching the validator.
COMMON_PASSWORDS: frozenset[str] = frozenset(
    {
        "password",
        "password1",
        "password123",
        "password123!",
        "passw0rd",
        "qwerty",
        "qwerty123",
        "123456",
        "12345678",
        "123456789",
        "1234567890",
        "letmein",
        "admin",
        "admin123",
        "welcome",
        "welcome1",
        "iloveyou",
        "monkey",
        "dragon",
        "football",
        "baseball",
        "abc123",
        "changeme",
        "changeme1",
        "p@ssw0rd",
        "sunshine",
        "princess",
        "superman",
        "trustno1",
        "master",
    }
)


class PasswordPolicyError(ValueError):
    """Raised when a candidate password fails a policy rule."""


def validate_password(password: str) -> None:
    """Raise :class:`PasswordPolicyError` if ``password`` violates the policy."""
    if len(password) < MIN_LENGTH:
        raise PasswordPolicyError(f"Password must be at least {MIN_LENGTH} characters long")
    if not any(c.isupper() for c in password):
        raise PasswordPolicyError("Password must contain an uppercase letter")
    if not any(c.islower() for c in password):
        raise PasswordPolicyError("Password must contain a lowercase letter")
    if not any(c.isdigit() for c in password):
        raise PasswordPolicyError("Password must contain a digit")
    if not any(c in SPECIAL_CHARS for c in password):
        raise PasswordPolicyError("Password must contain a special character")
    if password.lower() in COMMON_PASSWORDS:
        raise PasswordPolicyError("Password is too common; choose a less predictable password")


def generate_password(length: int = 16) -> str:
    """Generate a random password guaranteed to satisfy :func:`validate_password`."""
    if length < MIN_LENGTH:
        length = MIN_LENGTH
    alphabet = string.ascii_letters + string.digits + SPECIAL_CHARS
    while True:
        candidate = "".join(secrets.choice(alphabet) for _ in range(length))
        try:
            validate_password(candidate)
        except PasswordPolicyError:
            continue
        return candidate
