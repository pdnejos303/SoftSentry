"""CVE ingestion from NVD (Module 5.1).

Split into pure parsing (NVD 2.0 JSON → normalized record) and async I/O
(fetch pages, upsert into `cve_records`). The pure half is exhaustively unit
tested; the network half is exercised against a fake transport.

NVD 2.0 API: https://services.nvd.nist.gov/rest/json/cves/2.0
- 5 req / 30s without key, 50 / 30s with `NVD_API_KEY`.
- `lastModStartDate`/`lastModEndDate` give an incremental window.
"""

from __future__ import annotations

import asyncio
from datetime import UTC, datetime, timedelta
from typing import TYPE_CHECKING, Any

from sqlalchemy import select

from app.models.cve import CveRecord

if TYPE_CHECKING:
    import httpx
    from sqlalchemy.ext.asyncio import AsyncSession

NVD_API_URL = "https://services.nvd.nist.gov/rest/json/cves/2.0"
_PAGE_SIZE = 2000  # NVD max resultsPerPage
_REJECTED_STATUSES = {"Rejected", "Withdrawn"}
_SEVERITY_METRICS = ("cvssMetricV31", "cvssMetricV30", "cvssMetricV2")


# ── Pure parsing ──────────────────────────────────────────────────────────────


def parse_cpe_criteria(criteria: str) -> tuple[str, str, str]:
    """Extract (vendor, product, version) from a CPE 2.3 string.

    `*` / `-` version placeholders normalize to "". Malformed input → blanks.
    """
    parts = criteria.split(":")
    if len(parts) < 6 or parts[0] != "cpe" or parts[1] != "2.3":
        return "", "", ""
    vendor, product, version = parts[3], parts[4], parts[5]
    if version in ("*", "-"):
        version = ""
    return vendor, product, version


def extract_severity(metrics: dict[str, Any]) -> tuple[str, float | None]:
    """Pick the best available CVSS severity + score (v3.1 > v3.0 > v2)."""
    for key in _SEVERITY_METRICS:
        entries = metrics.get(key)
        if not entries:
            continue
        entry = entries[0]
        data = entry.get("cvssData", {})
        score = data.get("baseScore")
        # v3 carries baseSeverity inside cvssData; v2 carries it on the metric.
        severity = data.get("baseSeverity") or entry.get("baseSeverity")
        if severity:
            return severity.lower(), float(score) if score is not None else None
    return "unknown", None


def _parse_ts(value: str | None) -> datetime:
    if not value:
        return datetime.now(tz=UTC)
    cleaned = value.rstrip("Z")
    try:
        dt = datetime.fromisoformat(cleaned)
    except ValueError:
        return datetime.now(tz=UTC)
    return dt.replace(tzinfo=UTC) if dt.tzinfo is None else dt


def _english_description(descriptions: list[dict[str, Any]]) -> str:
    for d in descriptions:
        if d.get("lang") == "en":
            return str(d.get("value", ""))
    return str(descriptions[0].get("value", "")) if descriptions else ""


def _affected_from_configurations(
    configs: list[dict[str, Any]],
) -> tuple[list[str], list[dict[str, object]]]:
    """Flatten NVD configuration nodes into (cpe_match_strings, affected entries)."""
    cpe_matches: list[str] = []
    affected: list[dict[str, object]] = []
    for config in configs:
        for node in config.get("nodes", []):
            for cm in node.get("cpeMatch", []):
                if not cm.get("vulnerable", False):
                    continue
                criteria = cm.get("criteria", "")
                vendor, product, version = parse_cpe_criteria(criteria)
                if not product:
                    continue
                cpe_matches.append(criteria)
                entry: dict[str, object] = {"product": product, "vendor": vendor}
                if version:
                    entry["version"] = version
                for src, dst in (
                    ("versionStartIncluding", "version_start_including"),
                    ("versionStartExcluding", "version_start_excluding"),
                    ("versionEndIncluding", "version_end_including"),
                    ("versionEndExcluding", "version_end_excluding"),
                ):
                    if cm.get(src) is not None:
                        entry[dst] = cm[src]
                affected.append(entry)
    return cpe_matches, affected


def parse_nvd_item(item: dict[str, Any]) -> dict[str, Any] | None:
    """Normalize one NVD 2.0 vulnerability object, or None if rejected/unusable."""
    cve = item.get("cve", {})
    cve_id = cve.get("id")
    if not cve_id:
        return None
    if cve.get("vulnStatus") in _REJECTED_STATUSES:
        return None

    severity, score = extract_severity(cve.get("metrics", {}))
    cpe_matches, affected = _affected_from_configurations(cve.get("configurations", []))

    return {
        "cve_id": cve_id,
        "source": "nvd",
        "published_at": _parse_ts(cve.get("published")),
        "modified_at": _parse_ts(cve.get("lastModified")),
        "severity": severity,
        "cvss_score": score,
        "description": _english_description(cve.get("descriptions", [])),
        "cpe_matches": cpe_matches,
        "affected": affected,
        "references": [r["url"] for r in cve.get("references", []) if r.get("url")],
    }


# ── Persistence ───────────────────────────────────────────────────────────────


async def upsert_records(session: AsyncSession, records: list[dict[str, Any]]) -> dict[str, int]:
    """Insert new CVEs / update changed ones. Skips rows whose `modified_at` is
    unchanged (spec: don't re-download unchanged CVEs). Caller commits."""
    inserted = updated = skipped = 0
    existing = {
        r.cve_id: r
        for r in (
            await session.execute(
                select(CveRecord).where(CveRecord.cve_id.in_([rec["cve_id"] for rec in records]))
            )
        ).scalars()
    }
    for rec in records:
        current = existing.get(rec["cve_id"])
        if current is None:
            session.add(CveRecord(**rec))
            inserted += 1
            continue
        if current.modified_at == rec["modified_at"]:
            skipped += 1
            continue
        for field, value in rec.items():
            setattr(current, field, value)
        updated += 1
    await session.flush()
    return {"inserted": inserted, "updated": updated, "skipped": skipped}


# ── Network fetch ─────────────────────────────────────────────────────────────


async def fetch_nvd_page(
    client: httpx.AsyncClient,
    *,
    start_index: int,
    last_modified: datetime | None,
    api_key: str,
) -> dict[str, Any]:
    params: dict[str, str | int] = {"resultsPerPage": _PAGE_SIZE, "startIndex": start_index}
    if last_modified is not None:
        params["lastModStartDate"] = last_modified.isoformat()
        params["lastModEndDate"] = datetime.now(tz=UTC).isoformat()
    headers = {"apiKey": api_key} if api_key else {}
    resp = await client.get(NVD_API_URL, params=params, headers=headers, timeout=60)
    resp.raise_for_status()
    data: dict[str, Any] = resp.json()
    return data


async def sync_nvd(
    session: AsyncSession,
    client: httpx.AsyncClient,
    *,
    last_modified: datetime | None = None,
    api_key: str = "",
    max_pages: int | None = None,
) -> dict[str, int]:
    """Page through the NVD feed, parse + upsert. `last_modified` set → incremental.

    Returns aggregate {inserted, updated, skipped, total}.
    """
    totals = {"inserted": 0, "updated": 0, "skipped": 0, "total": 0}
    start_index = 0
    page_num = 0
    throttle = 0.6 if api_key else 6.0  # respect NVD rate limits

    while True:
        data = await fetch_nvd_page(
            client, start_index=start_index, last_modified=last_modified, api_key=api_key
        )
        vulns = data.get("vulnerabilities", [])
        records = [r for item in vulns if (r := parse_nvd_item(item)) is not None]
        if records:
            counts = await upsert_records(session, records)
            for k in ("inserted", "updated", "skipped"):
                totals[k] += counts[k]
        totals["total"] += len(vulns)

        total_results = int(data.get("totalResults", 0))
        start_index += _PAGE_SIZE
        page_num += 1
        if start_index >= total_results or not vulns:
            break
        if max_pages is not None and page_num >= max_pages:
            break
        await asyncio.sleep(throttle)

    await session.commit()
    return totals


def default_incremental_window() -> datetime:
    """NVD incremental syncs look back a day to tolerate a missed run."""
    return datetime.now(tz=UTC) - timedelta(days=1)
