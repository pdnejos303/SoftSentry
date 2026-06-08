"""Unit tests for the pure CVE matching engine (Module 5.2)."""

from __future__ import annotations

import pytest
from app.services.cve_matching import (
    build_cpe,
    match_software,
    normalize_product,
    vendor_slug,
    version_in_range,
)

# ── version_in_range ──────────────────────────────────────────────────────────


def test_version_below_end_excluding_matches():
    assert version_in_range("100.0", {"version_end_excluding": "101.0"}) is True


def test_version_at_end_excluding_does_not_match():
    assert version_in_range("101.0", {"version_end_excluding": "101.0"}) is False


def test_version_above_end_excluding_does_not_match():
    assert version_in_range("102.0", {"version_end_excluding": "101.0"}) is False


def test_version_at_start_including_matches():
    assert (
        version_in_range(
            "100.0",
            {"version_start_including": "100.0", "version_end_excluding": "101.0"},
        )
        is True
    )


def test_version_below_start_including_does_not_match():
    assert (
        version_in_range(
            "99.0",
            {"version_start_including": "100.0", "version_end_excluding": "101.0"},
        )
        is False
    )


def test_version_start_excluding_boundary():
    rng = {"version_start_excluding": "1.0", "version_end_including": "2.0"}
    assert version_in_range("1.0", rng) is False
    assert version_in_range("1.5", rng) is True
    assert version_in_range("2.0", rng) is True


def test_exact_version_match():
    assert version_in_range("1.2.3", {"version": "1.2.3"}) is True
    assert version_in_range("1.2.4", {"version": "1.2.3"}) is False


def test_no_bounds_matches_any_version():
    # Whole product affected (general issue, no version data).
    assert version_in_range("anything-1.0", {}) is True


def test_non_semver_version_falls_back_to_string_equality():
    # Windows build-style version, unparseable as semver.
    assert version_in_range("10.0.26200.1", {"version": "10.0.26200.1"}) is True
    assert version_in_range("10.0.26200.1", {"version": "10.0.26200.2"}) is False


def test_unparseable_version_with_range_does_not_match():
    # Can't place an unparseable version inside a numeric range → conservative no-match.
    assert version_in_range("weird-build", {"version_end_excluding": "2.0"}) is False


# ── normalize_product / vendor_slug ───────────────────────────────────────────


def test_normalize_product_lowercases_and_trims():
    assert normalize_product("  Google Chrome  ") == "google chrome"


def test_vendor_slug_drops_corporate_suffix():
    assert vendor_slug("Google LLC") == "google"
    assert vendor_slug("Mozilla Corporation") == "mozilla"
    assert vendor_slug("Oracle Corp.") == "oracle"


def test_vendor_slug_handles_none():
    assert vendor_slug(None) == ""


def test_build_cpe_format():
    assert build_cpe("google", "chrome", "100.0") == "cpe:2.3:a:google:chrome:100.0:*:*:*:*:*:*:*"


# ── match_software ────────────────────────────────────────────────────────────


def _cve(affected: list[dict]) -> dict:
    return {"affected": affected}


def test_chrome_old_version_matches_high_confidence_with_vendor():
    cve = _cve(
        [
            {
                "product": "chrome",
                "vendor": "google",
                "version_end_excluding": "101.0",
            }
        ]
    )
    result = match_software("Google Chrome", "Google LLC", "100.0", cve)
    assert result is not None
    assert result["recommended_version"] == "101.0"
    assert result["match_confidence"] == "high"


def test_same_product_name_different_vendor_does_not_match():
    # "Express" web framework vs "Express" desktop app — vendor disambiguates.
    cve = _cve([{"product": "express", "vendor": "openjsf", "version_end_excluding": "5.0"}])
    result = match_software("Express", "Acme Desktop Inc", "1.0", cve)
    assert result is None


def test_up_to_date_version_does_not_match():
    cve = _cve([{"product": "chrome", "vendor": "google", "version_end_excluding": "101.0"}])
    assert match_software("Google Chrome", "Google LLC", "101.0", cve) is None


def test_match_without_publisher_is_medium_confidence():
    cve = _cve([{"product": "chrome", "vendor": "google", "version_end_excluding": "101.0"}])
    result = match_software("Google Chrome", None, "100.0", cve)
    assert result is not None
    assert result["match_confidence"] == "medium"


def test_product_only_match_is_low_confidence():
    cve = _cve([{"product": "chrome", "vendor": "google"}])  # no version range
    result = match_software("Google Chrome", "Google LLC", "100.0", cve)
    assert result is not None
    assert result["match_confidence"] == "low"


def test_unrelated_product_does_not_match():
    cve = _cve([{"product": "firefox", "vendor": "mozilla", "version_end_excluding": "120.0"}])
    assert match_software("Google Chrome", "Google LLC", "100.0", cve) is None


def test_picks_best_confidence_across_affected_entries():
    cve = _cve(
        [
            {"product": "chrome", "vendor": "google"},  # low (product only)
            {"product": "chrome", "vendor": "google", "version_end_excluding": "101.0"},  # high
        ]
    )
    result = match_software("Google Chrome", "Google LLC", "100.0", cve)
    assert result is not None
    assert result["match_confidence"] == "high"
    assert result["recommended_version"] == "101.0"


@pytest.mark.parametrize("missing", [None, ""])
def test_empty_affected_yields_no_match(missing):
    assert match_software("Anything", "Vendor", "1.0", {"affected": missing}) is None


# ── product identity (false-positive mitigation, spec 5.2) ────────────────────


def test_bare_product_does_not_leak_onto_different_product():
    # A Google Chrome *browser* CVE (product "chrome", vendor "google") must not
    # attach to "Chrome Remote Desktop Host": the product is only a *leading*
    # token of the name, same vendor, overlapping 149.x versioning.
    cve = _cve([{"product": "chrome", "vendor": "google", "version_end_excluding": "150.0"}])
    assert match_software("Chrome Remote Desktop Host", "Google LLC", "149.0", cve) is None


def test_vendor_prefixed_display_name_matches_product_suffix():
    # "Google Chrome" ⊃ product "chrome" as a trailing suffix → still a real match.
    cve = _cve([{"product": "chrome", "vendor": "google", "version_end_excluding": "150.0"}])
    result = match_software("Google Chrome", "Google LLC", "149.0", cve)
    assert result is not None
    assert result["match_confidence"] == "high"


def test_descriptor_suffix_name_does_not_match_bare_product():
    # "Python Launcher" (py.exe) is a distinct product from CPython ("python");
    # its own 3.12.x build number must not pull in interpreter CVEs.
    cve = _cve([{"product": "python", "vendor": "python", "version_end_excluding": "3.13"}])
    assert match_software("Python Launcher", "Python Software Foundation", "3.12", cve) is None
