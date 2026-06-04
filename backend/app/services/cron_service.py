"""Cron expression validation + next-run calculation (spec 8.5).

Wraps `croniter`. All times are treated as UTC (the server TZ); the schedule UI
displays the TZ explicitly. Used by the schedule worker to compute `next_run_at`
and by the API to preview upcoming runs.
"""

from __future__ import annotations

from datetime import UTC, datetime

from croniter import croniter


class InvalidCron(ValueError):
    """Raised when a cron expression does not parse."""


def validate(expr: str) -> str:
    """Return the trimmed expression if valid, else raise InvalidCron."""
    trimmed = expr.strip()
    if not trimmed or not croniter.is_valid(trimmed):
        raise InvalidCron(f"Invalid cron expression: {expr!r}")
    return trimmed


def _base(after: datetime | None) -> datetime:
    base = after or datetime.now(tz=UTC)
    return base if base.tzinfo is not None else base.replace(tzinfo=UTC)


def next_run(expr: str, *, after: datetime | None = None) -> datetime:
    """The first fire time strictly after `after` (default: now), in UTC."""
    itr = croniter(validate(expr), _base(after))
    result: datetime = itr.get_next(datetime)
    return result


def next_runs(expr: str, *, count: int = 3, after: datetime | None = None) -> list[datetime]:
    """The next `count` fire times — used for the UI's preview."""
    itr = croniter(validate(expr), _base(after))
    return [itr.get_next(datetime) for _ in range(count)]
