"""CSV parsing for bulk policy import (Module 4.3)."""

from __future__ import annotations

from app.services.policy_csv import parse_policy_csv

WL_HEADER = "name_pattern,publisher_pattern,version_constraint,notes\n"
BL_HEADER = "name_pattern,publisher_pattern,version_constraint,notes,severity,reason\n"


class TestWhitelistCsv:
    def test_valid_rows(self):
        data = (
            WL_HEADER
            + "Microsoft Office %,Microsoft Corporation,,Office suite\nAdobe%,Adobe,,Design\n"
        ).encode()
        rows, errors = parse_policy_csv(data, kind="whitelist")
        assert errors == []
        assert len(rows) == 2
        assert rows[0]["name_pattern"] == "Microsoft Office %"
        assert rows[0]["publisher_pattern"] == "Microsoft Corporation"
        assert rows[0]["version_constraint"] is None
        assert rows[0]["notes"] == "Office suite"

    def test_strips_utf8_bom(self):
        data = ("﻿" + WL_HEADER + "Chrome%,Google,,\n").encode("utf-8")
        rows, errors = parse_policy_csv(data, kind="whitelist")
        assert errors == []
        assert len(rows) == 1
        assert rows[0]["name_pattern"] == "Chrome%"

    def test_missing_name_pattern_is_row_error(self):
        data = (WL_HEADER + "Valid%,,,\n,Google,,no name\n").encode()
        rows, errors = parse_policy_csv(data, kind="whitelist")
        assert len(rows) == 1
        assert len(errors) == 1
        assert errors[0].row == 2  # second data row
        assert "name_pattern" in errors[0].message

    def test_bad_header_returns_error_no_rows(self):
        data = b"wrong,header\nfoo,bar\n"
        rows, errors = parse_policy_csv(data, kind="whitelist")
        assert rows == []
        assert len(errors) == 1
        assert errors[0].row == 0

    def test_empty_file(self):
        rows, errors = parse_policy_csv(b"", kind="whitelist")
        assert rows == []
        assert len(errors) == 1

    def test_quoted_field_with_comma(self):
        data = (WL_HEADER + '"Acme, Inc Tool",Acme,,"notes, with comma"\n').encode()
        rows, errors = parse_policy_csv(data, kind="whitelist")
        assert errors == []
        assert rows[0]["name_pattern"] == "Acme, Inc Tool"
        assert rows[0]["notes"] == "notes, with comma"


class TestBlacklistCsv:
    def test_valid_with_severity_and_reason(self):
        data = (BL_HEADER + "uTorrent,,,,high,P2P prohibited\n").encode()
        rows, errors = parse_policy_csv(data, kind="blacklist")
        assert errors == []
        assert rows[0]["severity"] == "high"
        assert rows[0]["reason"] == "P2P prohibited"

    def test_missing_reason_is_error(self):
        data = (BL_HEADER + "uTorrent,,,,high,\n").encode()
        rows, errors = parse_policy_csv(data, kind="blacklist")
        assert rows == []
        assert len(errors) == 1
        assert "reason" in errors[0].message

    def test_blank_severity_defaults_high(self):
        data = (BL_HEADER + "BadApp,,,,,Not allowed\n").encode()
        rows, errors = parse_policy_csv(data, kind="blacklist")
        assert errors == []
        assert rows[0]["severity"] == "high"

    def test_invalid_severity_is_error(self):
        data = (BL_HEADER + "BadApp,,,,critical,Not allowed\n").encode()
        rows, errors = parse_policy_csv(data, kind="blacklist")
        assert rows == []
        assert len(errors) == 1
        assert "severity" in errors[0].message
