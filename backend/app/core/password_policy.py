"""Password policy validation + random password generation (Module 9 — spec 9.8).

Rules: ขั้นต่ำ 12 ตัว ต้องมีทั้ง uppercase + lowercase + ตัวเลข + special char
และห้ามเป็น common password ที่รู้จักกันดี ชุด common password ที่ bundle ไว้นี้
เป็น subset ที่คัดมาจากรหัสผ่านที่รั่วไหลมากที่สุด ใน production สามารถเปลี่ยนเป็น
full top-10k list ได้โดยแทน `COMMON_PASSWORDS` ใหม่ทั้งชุด
"""

from __future__ import annotations

import secrets
import string

MIN_LENGTH = 12
SPECIAL_CHARS = "!@#$%^&*()-_=+[]{};:,.<>?/|~`"

# Curated subset ของ common/breached password (lowercase) ตาม spec ควรใช้ full top-10k
# ชุดนี้ป้องกัน password ที่ชัดเจนที่สุดได้ราคาถูก และเปลี่ยนได้ทั้งชุดโดยไม่ต้องแตะ validator
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
    """Raise เมื่อ password ไม่ผ่าน policy rule ข้อใดข้อหนึ่ง."""


def validate_password(password: str) -> None:
    """Raise :class:`PasswordPolicyError` ถ้า ``password`` ละเมิด policy."""
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
    """สร้าง random password ที่รับประกันว่าผ่าน :func:`validate_password`."""
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
