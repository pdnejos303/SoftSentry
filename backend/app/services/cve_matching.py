"""Pure CVE matching engine (Module 5.2).

No I/O — given an installed software (name, publisher, version) and a CVE's
normalized `affected` list, decide whether the software is vulnerable and with
what confidence. Kept side-effect-free so it is exhaustively unit-testable and
can be reused both inline (scan ingest) and in the rematch worker.
"""

from __future__ import annotations

import re
from itertools import pairwise

from packaging.version import InvalidVersion, Version

# Corporate suffixes stripped when slugging a publisher into an NVD-style vendor.
_VENDOR_SUFFIXES = {
    "inc",
    "incorporated",
    "llc",
    "ltd",
    "limited",
    "corp",
    "corporation",
    "co",
    "gmbh",
    "ab",
    "sa",
    "plc",
    "software",
    "technologies",
    "systems",
}

_CONFIDENCE_RANK = {"high": 3, "medium": 2, "low": 1}


def normalize_product(value: str | None) -> str:
    """Lowercase + collapse internal whitespace for product-name comparison."""
    if not value:
        return ""
    return re.sub(r"\s+", " ", value.strip().lower())


def candidate_product_keys(name: str) -> set[str]:
    """Index keys for a software name, used to pre-filter CVE candidates.

    Emits the full normalized name, each word, and adjacent word bigrams so a
    CVE keyed by its (typically single- or two-word) product is found without
    scanning every record. The authoritative test stays `match_software`.
    """
    norm = normalize_product(name)
    if not norm:
        return set()
    words = norm.split(" ")
    keys = {norm, *words}
    keys.update(f"{a} {b}" for a, b in pairwise(words))
    return keys


def vendor_slug(publisher: str | None) -> str:
    """Best-effort NVD-style vendor slug from a publisher string.

    "Google LLC" → "google", "Mozilla Corporation" → "mozilla". Drops trailing
    corporate suffixes and punctuation; joins remaining tokens with '_'.
    """
    if not publisher:
        return ""
    tokens = [t for t in re.split(r"[^a-z0-9]+", publisher.lower()) if t]
    kept = [t for t in tokens if t not in _VENDOR_SUFFIXES]
    if not kept:
        kept = tokens
    return "_".join(kept)


def build_cpe(vendor: str, product: str, version: str) -> str:
    """Build a CPE 2.3 application string (used for the Postgres GIN pre-filter)."""
    return f"cpe:2.3:a:{vendor}:{product}:{version}:*:*:*:*:*:*:*"


def _parse(value: str | None) -> Version | None:
    if value is None:
        return None
    try:
        return Version(value)
    except (InvalidVersion, TypeError):
        return None


def version_in_range(version: str, rng: dict[str, object]) -> bool:
    """True if `version` falls within an NVD-style version range.

    Range keys (all optional): version_start_including, version_start_excluding,
    version_end_including, version_end_excluding, and `version` (exact). With no
    keys, the whole product is affected → True. Non-parseable versions fall back
    to exact string equality; they never match an open numeric range.
    """
    exact = rng.get("version")
    start_inc = rng.get("version_start_including")
    start_exc = rng.get("version_start_excluding")
    end_inc = rng.get("version_end_including")
    end_exc = rng.get("version_end_excluding")

    has_bounds = any(b is not None for b in (start_inc, start_exc, end_inc, end_exc))

    if exact is not None and not has_bounds:
        if str(exact) == version:
            return True
        ev, pv = _parse(str(exact)), _parse(version)
        return ev is not None and pv is not None and ev == pv

    if not has_bounds:
        # No version constraints at all → entire product is affected.
        return True

    pv = _parse(version)
    if pv is None:
        return False  # unparseable version can't be placed in a numeric range

    if start_inc is not None and (sv := _parse(str(start_inc))) is not None and pv < sv:
        return False
    if start_exc is not None and (sv := _parse(str(start_exc))) is not None and pv <= sv:
        return False
    if end_inc is not None and (ev := _parse(str(end_inc))) is not None and pv > ev:
        return False
    if end_exc is not None and (ev := _parse(str(end_exc))) is not None and pv >= ev:  # noqa: SIM103
        return False
    return True


def _product_match(sw_name: str, product: str) -> bool:
    """True if the CVE product identifies this software.

    Matches when the normalized product equals the software name, or is a
    trailing token-suffix of it — the vendor-prefixed display-name case where
    NVD keys the bare product ("chrome") but the installed name carries the
    vendor ("Google Chrome").

    A product that is only a *leading* or interior token of the name is rejected.
    This is the key false-positive guard (spec 5.2): "chrome" (Google Chrome
    browser) must NOT attach to a different product whose name merely starts
    with it — "Chrome Remote Desktop Host", "Python Launcher" vs "python" —
    even when vendor and version overlap.
    """
    n = [t for t in normalize_product(sw_name).split(" ") if t]
    p = [t for t in normalize_product(product).split(" ") if t]
    if not n or not p:
        return False
    if n == p:
        return True
    return len(p) < len(n) and n[-len(p) :] == p


def _vendor_conflict(sw_publisher: str | None, vendor: str | None) -> bool:
    """True if both vendors are known and clearly disagree (neither contains the
    other) — the key false-positive guard (spec: Express-vs-Express)."""
    if not sw_publisher or not vendor:
        return False
    sw = vendor_slug(sw_publisher)
    cv = vendor_slug(vendor)
    if not sw or not cv:
        return False
    return sw != cv and sw not in cv and cv not in sw


def _has_version_data(entry: dict[str, object]) -> bool:
    return any(
        entry.get(k) is not None
        for k in (
            "version",
            "version_start_including",
            "version_start_excluding",
            "version_end_including",
            "version_end_excluding",
        )
    )


def match_software(
    name: str,
    publisher: str | None,
    version: str,
    cve: dict[str, object],
) -> dict[str, object] | None:
    """Return the best match for this software against one CVE, or None.

    Result: {match_confidence: high|medium|low, recommended_version: str|None}.
    Confidence (spec 5.2): name+vendor+version exact → high; name+version
    (vendor unconfirmed) → medium; product-only (no version data) → low.
    """
    affected = cve.get("affected")
    if not affected or not isinstance(affected, list):
        return None

    best: dict[str, object] | None = None
    for entry in affected:
        if not isinstance(entry, dict):
            continue
        product = entry.get("product")
        if not isinstance(product, str) or not _product_match(name, product):
            continue
        vendor = entry.get("vendor")
        if _vendor_conflict(publisher, vendor if isinstance(vendor, str) else None):
            continue
        if not version_in_range(version, entry):
            continue

        vendor_confirmed = (
            bool(publisher)
            and bool(vendor)
            and not _vendor_conflict(publisher, vendor if isinstance(vendor, str) else None)
        )
        if not _has_version_data(entry):
            confidence = "low"
        elif vendor_confirmed:
            confidence = "high"
        else:
            confidence = "medium"

        candidate = {
            "match_confidence": confidence,
            "recommended_version": entry.get("version_end_excluding"),
        }
        if (
            best is None
            or _CONFIDENCE_RANK[confidence] > _CONFIDENCE_RANK[str(best["match_confidence"])]
        ):
            best = candidate

    return best
