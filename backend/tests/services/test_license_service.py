"""Module 6 — pure license helpers + column encryption (no DB)."""

from __future__ import annotations

from datetime import date, timedelta

from app.core.crypto import decrypt_secret, encrypt_secret
from app.services import license_service as ls

TODAY = date(2026, 6, 3)


def _in(days: int) -> date:
    return TODAY + timedelta(days=days)


class TestStatus:
    def test_compliant_when_under_purchased_and_not_expiring(self) -> None:
        assert ls.compute_status(5, 20, _in(400), TODAY) == ls.STATUS_COMPLIANT

    def test_perpetual_license_never_expires(self) -> None:
        assert ls.compute_status(5, 20, None, TODAY) == ls.STATUS_COMPLIANT

    def test_over_used_when_installed_exceeds_purchased(self) -> None:
        assert ls.compute_status(25, 20, _in(400), TODAY) == ls.STATUS_OVER_USED

    def test_expiring_soon_within_90_days(self) -> None:
        assert ls.compute_status(5, 20, _in(25), TODAY) == ls.STATUS_EXPIRING

    def test_expired_yesterday(self) -> None:
        assert ls.compute_status(5, 20, _in(-1), TODAY) == ls.STATUS_EXPIRED

    def test_expired_outranks_over_used(self) -> None:
        assert ls.compute_status(25, 20, _in(-1), TODAY) == ls.STATUS_EXPIRED


class TestDaysAndBuckets:
    def test_days_until_none_for_perpetual(self) -> None:
        assert ls.days_until(None, TODAY) is None

    def test_days_until_counts_calendar_days(self) -> None:
        assert ls.days_until(_in(30), TODAY) == 30

    def test_subbucket_30_60_90(self) -> None:
        assert ls.expiring_subbucket(25) == 30
        assert ls.expiring_subbucket(45) == 60
        assert ls.expiring_subbucket(80) == 90

    def test_severity_escalates_toward_expiry(self) -> None:
        assert ls.expiry_severity(5) == "high"
        assert ls.expiry_severity(25) == "medium"
        assert ls.expiry_severity(80) == "low"


class TestEncryption:
    def test_round_trip(self) -> None:
        cipher = encrypt_secret("SECRET-KEY-123")
        assert cipher is not None
        assert cipher != b"SECRET-KEY-123"  # not plaintext
        assert decrypt_secret(cipher) == "SECRET-KEY-123"

    def test_empty_inputs_map_to_none(self) -> None:
        assert encrypt_secret(None) is None
        assert encrypt_secret("") is None
        assert decrypt_secret(None) is None

    def test_special_chars_survive(self) -> None:
        secret = "k€y-with-üñî©ode-\n-and-tabs\t!"
        cipher = encrypt_secret(secret)
        assert decrypt_secret(cipher) == secret
