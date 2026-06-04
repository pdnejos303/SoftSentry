"""Module 8.5 — cron validation + next-run calculation."""

from __future__ import annotations

from datetime import UTC, datetime

import pytest
from app.services import cron_service


def test_validate_accepts_standard_cron() -> None:
    assert cron_service.validate(" 0 9 * * MON ") == "0 9 * * MON"


def test_validate_rejects_garbage() -> None:
    with pytest.raises(cron_service.InvalidCron):
        cron_service.validate("not a cron")


def test_validate_rejects_empty() -> None:
    with pytest.raises(cron_service.InvalidCron):
        cron_service.validate("   ")


def test_next_run_is_following_monday_9am() -> None:
    after = datetime(2026, 6, 3, 12, 0, tzinfo=UTC)  # a Wednesday
    nxt = cron_service.next_run("0 9 * * MON", after=after)
    assert nxt.weekday() == 0  # Monday
    assert (nxt.hour, nxt.minute) == (9, 0)
    assert nxt > after


def test_next_runs_returns_increasing_sequence() -> None:
    runs = cron_service.next_runs("0 9 * * *", count=3)
    assert len(runs) == 3
    assert runs[0] < runs[1] < runs[2]


def test_naive_after_is_treated_as_utc() -> None:
    nxt = cron_service.next_run("0 0 * * *", after=datetime(2026, 6, 3, 1, 0))
    assert nxt.tzinfo is not None
