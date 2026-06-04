"""Unit tests for NVD 2.0 parsing (Module 5.1)."""

from __future__ import annotations

from app.services.cve_sync import (
    extract_severity,
    parse_cpe_criteria,
    parse_nvd_item,
)


def test_parse_cpe_criteria_extracts_vendor_product_version():
    vendor, product, version = parse_cpe_criteria("cpe:2.3:a:google:chrome:100.0:*:*:*:*:*:*:*")
    assert (vendor, product, version) == ("google", "chrome", "100.0")


def test_parse_cpe_criteria_wildcard_version_is_empty():
    vendor, product, version = parse_cpe_criteria("cpe:2.3:a:google:chrome:*:*:*:*:*:*:*:*")
    assert (vendor, product) == ("google", "chrome")
    assert version == ""


def test_parse_cpe_criteria_invalid_returns_blanks():
    assert parse_cpe_criteria("not-a-cpe") == ("", "", "")


def test_extract_severity_prefers_v31():
    metrics = {
        "cvssMetricV31": [{"cvssData": {"baseScore": 9.8, "baseSeverity": "CRITICAL"}}],
        "cvssMetricV2": [{"baseSeverity": "MEDIUM", "cvssData": {"baseScore": 5.0}}],
    }
    assert extract_severity(metrics) == ("critical", 9.8)


def test_extract_severity_falls_back_to_v2():
    metrics = {"cvssMetricV2": [{"baseSeverity": "HIGH", "cvssData": {"baseScore": 7.5}}]}
    assert extract_severity(metrics) == ("high", 7.5)


def test_extract_severity_unknown_when_no_metrics():
    assert extract_severity({}) == ("unknown", None)


_ITEM = {
    "cve": {
        "id": "CVE-2024-0001",
        "published": "2024-01-15T10:00:00.000",
        "lastModified": "2024-02-01T12:00:00.000",
        "vulnStatus": "Analyzed",
        "descriptions": [
            {"lang": "en", "value": "Heap overflow in Chrome."},
            {"lang": "es", "value": "ignored"},
        ],
        "metrics": {
            "cvssMetricV31": [{"cvssData": {"baseScore": 9.8, "baseSeverity": "CRITICAL"}}]
        },
        "references": [{"url": "https://example.com/a"}, {"url": "https://example.com/b"}],
        "configurations": [
            {
                "nodes": [
                    {
                        "cpeMatch": [
                            {
                                "vulnerable": True,
                                "criteria": "cpe:2.3:a:google:chrome:*:*:*:*:*:*:*:*",
                                "versionEndExcluding": "101.0",
                            }
                        ]
                    }
                ]
            }
        ],
    }
}


def test_parse_nvd_item_full():
    rec = parse_nvd_item(_ITEM)
    assert rec is not None
    assert rec["cve_id"] == "CVE-2024-0001"
    assert rec["severity"] == "critical"
    assert float(rec["cvss_score"]) == 9.8
    assert "Heap overflow" in rec["description"]
    assert rec["references"] == ["https://example.com/a", "https://example.com/b"]
    assert "cpe:2.3:a:google:chrome:*:*:*:*:*:*:*:*" in rec["cpe_matches"]
    affected = rec["affected"]
    assert affected[0]["product"] == "chrome"
    assert affected[0]["vendor"] == "google"
    assert affected[0]["version_end_excluding"] == "101.0"


def test_parse_nvd_item_rejects_withdrawn():
    item = {"cve": {"id": "CVE-2024-9999", "vulnStatus": "Rejected"}}
    assert parse_nvd_item(item) is None


def test_parse_nvd_item_keeps_exact_version_from_cpe():
    item = {
        "cve": {
            "id": "CVE-2024-0002",
            "published": "2024-01-01T00:00:00.000",
            "lastModified": "2024-01-01T00:00:00.000",
            "vulnStatus": "Analyzed",
            "descriptions": [{"lang": "en", "value": "x"}],
            "metrics": {},
            "configurations": [
                {
                    "nodes": [
                        {
                            "cpeMatch": [
                                {
                                    "vulnerable": True,
                                    "criteria": "cpe:2.3:a:acme:tool:1.2.3:*:*:*:*:*:*:*",
                                }
                            ]
                        }
                    ]
                }
            ],
        }
    }
    rec = parse_nvd_item(item)
    assert rec is not None
    assert rec["severity"] == "unknown"
    assert rec["affected"][0]["version"] == "1.2.3"
