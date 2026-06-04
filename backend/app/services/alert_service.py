"""Listing + lifecycle transitions for alerts (shared across detection modules)."""

from __future__ import annotations

import uuid as uuid_lib
from datetime import UTC, datetime
from typing import TYPE_CHECKING

from sqlalchemy import ColumnElement, func, select

from app.models.alert import (
    STATUS_ACKNOWLEDGED,
    STATUS_RESOLVED,
    Alert,
)
from app.models.license import License
from app.models.machine import Machine
from app.models.software import SoftwareRecord

if TYPE_CHECKING:
    from sqlalchemy.ext.asyncio import AsyncSession


async def list_alerts(
    session: AsyncSession,
    *,
    types: list[str] | None = None,
    status: str | None = None,
    severity: str | None = None,
    machine_uuid: uuid_lib.UUID | None = None,
    page: int = 1,
    page_size: int = 50,
) -> tuple[list[dict[str, object]], int]:
    stmt = (
        select(
            Alert,
            Machine.uuid,
            Machine.hostname,
            SoftwareRecord.name,
            SoftwareRecord.version,
            License.uuid,
            License.software_name,
        )
        # Outer joins: license alerts have no machine/software; CVE/policy alerts
        # have no license.
        .outerjoin(Machine, Alert.machine_id == Machine.id)
        .outerjoin(SoftwareRecord, Alert.software_id == SoftwareRecord.id)
        .outerjoin(License, Alert.license_id == License.id)
    )
    count_stmt = select(func.count()).select_from(Alert)

    conds: list[ColumnElement[bool]] = []
    if types:
        conds.append(Alert.type.in_(types))
    if status:
        conds.append(Alert.status == status)
    if severity:
        conds.append(Alert.severity == severity)
    if machine_uuid is not None:
        # Filter by machine uuid via subquery to keep count_stmt simple.
        sub = select(Machine.id).where(Machine.uuid == machine_uuid).scalar_subquery()
        conds.append(Alert.machine_id == sub)
    for c in conds:
        stmt = stmt.where(c)
        count_stmt = count_stmt.where(c)

    total = (await session.execute(count_stmt)).scalar_one()
    stmt = stmt.order_by(Alert.created_at.desc()).offset((page - 1) * page_size).limit(page_size)
    rows = (await session.execute(stmt)).all()

    items = [
        {
            "uuid": alert.uuid,
            "machine_uuid": m_uuid,
            "machine_hostname": hostname,
            "software_name": sw_name,
            "software_version": sw_version,
            "license_uuid": lic_uuid,
            "license_name": lic_name,
            "type": alert.type,
            "severity": alert.severity,
            "status": alert.status,
            "title": alert.title,
            "detail": alert.detail,
            "created_at": alert.created_at,
            "acknowledged_at": alert.acknowledged_at,
            "resolved_at": alert.resolved_at,
        }
        for alert, m_uuid, hostname, sw_name, sw_version, lic_uuid, lic_name in rows
    ]
    return items, total


async def get_alert(session: AsyncSession, alert_uuid: uuid_lib.UUID) -> Alert | None:
    return (
        await session.execute(select(Alert).where(Alert.uuid == alert_uuid))
    ).scalar_one_or_none()


async def to_dict(session: AsyncSession, alert: Alert) -> dict[str, object]:
    """Build the AlertOut payload for a single alert."""
    machine = await session.get(Machine, alert.machine_id) if alert.machine_id else None
    sw = await session.get(SoftwareRecord, alert.software_id) if alert.software_id else None
    lic = await session.get(License, alert.license_id) if alert.license_id else None
    return {
        "uuid": alert.uuid,
        "machine_uuid": machine.uuid if machine else None,
        "machine_hostname": machine.hostname if machine else None,
        "software_name": sw.name if sw else None,
        "software_version": sw.version if sw else None,
        "license_uuid": lic.uuid if lic else None,
        "license_name": lic.software_name if lic else None,
        "type": alert.type,
        "severity": alert.severity,
        "status": alert.status,
        "title": alert.title,
        "detail": alert.detail,
        "created_at": alert.created_at,
        "acknowledged_at": alert.acknowledged_at,
        "resolved_at": alert.resolved_at,
    }


def acknowledge(alert: Alert) -> None:
    alert.status = STATUS_ACKNOWLEDGED
    alert.acknowledged_at = datetime.now(tz=UTC)


def resolve(alert: Alert) -> None:
    alert.status = STATUS_RESOLVED
    alert.resolved_at = datetime.now(tz=UTC)
