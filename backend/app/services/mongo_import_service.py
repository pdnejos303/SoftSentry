from __future__ import annotations

from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from typing import TYPE_CHECKING, Any

from app.core.config import settings
from app.core.db import SessionLocal
from app.core.logging import logger
from app.core.mongo import get_source_collection
from app.schemas.scans import ScanIn, SignatureIn, SoftwareIn
from app.services import (
    agentless_service,
    risk_service,
    scan_service,
    signature_service,
    unauthorized_service,
    vulnerability_service,
)

if TYPE_CHECKING:
    from sqlalchemy.ext.asyncio import AsyncSession


@dataclass
class ImportResult:
    documents_seen: int = 0
    machines_imported: int = 0
    errors: int = 0


def _parse_dt(value: Any) -> datetime:
    """Coerce a Mongo value (datetime or ISO string) to an aware UTC datetime."""
    if isinstance(value, datetime):
        return value if value.tzinfo else value.replace(tzinfo=UTC)
    if isinstance(value, str):
        return datetime.fromisoformat(value.replace("Z", "+00:00"))
    return datetime.now(tz=UTC)


def _map_signature(raw: dict[str, Any] | None) -> SignatureIn | None:
    if not raw:
        return None
    return SignatureIn(
        status=raw.get("status", "unsigned"),
        signer=raw.get("signer"),
        issuer=raw.get("issuer"),
        cert_thumbprint=raw.get("cert_thumbprint"),
        cert_valid_from=raw.get("cert_valid_from"),
        cert_valid_to=raw.get("cert_valid_to"),
        signature_algorithm=raw.get("signature_algorithm"),
        chain=raw.get("chain"),
    )


def _map_software(raw: dict[str, Any]) -> SoftwareIn:
    """Map one source software entry → SoftwareIn. Adjust keys to your schema."""
    return SoftwareIn(
        name=raw["name"],
        version=str(raw.get("version", "")),
        publisher=raw.get("publisher"),
        install_date=raw.get("install_date"),
        install_path=raw.get("install_path"),
        install_size_kb=raw.get("install_size_kb"),
        arch=raw.get("arch"),
        source=raw.get("source", "registry"),
        signature=_map_signature(raw.get("signature")),
    )


def _map_scan(doc: dict[str, Any]) -> ScanIn:
    """Map a whole Mongo document → ScanIn (the inventory payload)."""
    scanned_at = _parse_dt(doc.get(settings.mongo_timestamp_field))
    software = [_map_software(s) for s in doc.get("software", [])]
    return ScanIn(
        started_at=scanned_at,
        completed_at=scanned_at,
        scan_type="auto",
        trigger="mongo-import",
        software=software,
    )


async def _import_one(session: AsyncSession, doc: dict[str, Any]) -> None:
    """Ingest a single Mongo document within the caller's transaction."""
    machine = await agentless_service.get_or_create_machine(
        session=session,
        hostname=doc["hostname"],
        os=doc.get("os", "unknown"),
        os_version=doc.get("os_version", "unknown"),
        arch=doc.get("arch", "unknown"),
    )

    payload = _map_scan(doc)
    # Re-importing the same Mongo doc version must be a no-op: key on _id + ts.
    idem = f"mongo:{doc['_id']}:{payload.completed_at.isoformat()}"

    await scan_service.process_scan(
        session=session,
        machine=machine,
        payload=payload,
        idempotency_key=idem,
    )
    # Same detection chain the agent endpoint runs, same order.
    await unauthorized_service.evaluate_machine(session, machine)
    await signature_service.evaluate_machine(session, machine)
    await vulnerability_service.evaluate_machine(session, machine)
    await risk_service.recompute_machine(session, machine)


async def run_import(*, lookback_minutes: int | None = None) -> ImportResult:
    """Pull recent documents from Mongo and ingest them. Returns a summary.

    Each document is committed in its own transaction so one bad document does
    not roll back the whole batch.
    """
    if not settings.mongo_import_enabled:
        logger.info("mongo_import.disabled")
        return ImportResult()

    lookback = (
        settings.mongo_import_lookback_minutes
        if lookback_minutes is None
        else lookback_minutes
    )
    since = datetime.now(tz=UTC) - timedelta(minutes=lookback)
    collection = get_source_collection()

    since_str = since.strftime("%Y-%m-%dT%H:%M:%SZ")
    query = {settings.mongo_timestamp_field: {"$gte": since_str}}
    result = ImportResult()

    cursor = collection.find(query).sort(settings.mongo_timestamp_field, 1)
    async for doc in cursor:
        result.documents_seen += 1
        try:
            async with SessionLocal() as session:
                await _import_one(session, doc)
                await session.commit()
            result.machines_imported += 1
        except Exception:
            result.errors += 1
            logger.exception(
                "mongo_import.document_failed",
                mongo_id=str(doc.get("_id")),
                hostname=doc.get("hostname"),
            )

    logger.info(
        "mongo_import.done",
        seen=result.documents_seen,
        imported=result.machines_imported,
        errors=result.errors,
    )
    return result