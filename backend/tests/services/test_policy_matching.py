"""Pattern-matching engine for whitelist/blacklist policy (Module 4)."""

from __future__ import annotations

import pytest
from app.services.policy_matching import ilike_match, matches, version_matches


class TestIlikeMatch:
    @pytest.mark.parametrize(
        ("value", "pattern", "expected"),
        [
            # plain text → exact, case-insensitive
            ("uTorrent", "utorrent", True),
            ("uTorrent", "BitTorrent", False),
            # % = 0+ chars
            ("Microsoft Office 2019", "Microsoft Office %", True),
            ("Microsoft Office 365", "Microsoft Office %", True),
            ("Microsoft Office", "Microsoft Office%", True),  # % matches empty (no literal space)
            ("Microsoft Office", "Microsoft Office %", False),  # literal space is required
            ("LibreOffice", "Microsoft Office %", False),
            ("BitTorrent 7.10", "BitTorrent%", True),
            # _ = exactly one char
            ("v1", "v_", True),
            ("v12", "v_", False),
            ("v1", "v__", False),
            # % in the middle
            ("Adobe Creative Cloud", "Adobe%Cloud", True),
            # regex metacharacters in pattern are literal
            ("a.b", "a.b", True),
            ("axb", "a.b", False),
        ],
    )
    def test_cases(self, value, pattern, expected):
        assert ilike_match(value, pattern) is expected

    def test_empty_value_against_wildcard(self):
        assert ilike_match("", "%") is True
        assert ilike_match("", "_") is False


class TestVersionMatches:
    def test_no_constraint_always_matches(self):
        assert version_matches("1.2.3", None) is True
        assert version_matches("anything", "") is True

    @pytest.mark.parametrize(
        ("version", "constraint", "expected"),
        [
            ("2.5", ">=2.0,<3.0", True),
            ("3.1", ">=2.0,<3.0", False),
            ("1.9", ">=2.0,<3.0", False),
            ("2.0", ">=2.0,<3.0", True),
        ],
    )
    def test_within_range(self, version, constraint, expected):
        assert version_matches(version, constraint) is expected

    def test_unparseable_version_matches_by_default(self):
        # Spec 4: version not semver-parseable → don't filter out (match).
        assert version_matches("2019-build-x", ">=2.0,<3.0") is True

    def test_invalid_constraint_matches_by_default(self):
        assert version_matches("1.0", "not-a-spec") is True


class TestMatches:
    def test_name_only(self):
        assert matches("uTorrent", None, "3.5", name_pattern="utorrent") is True
        assert matches("Chrome", None, "1.0", name_pattern="utorrent") is False

    def test_publisher_filter(self):
        assert (
            matches(
                "Office",
                "Microsoft Corporation",
                "1.0",
                name_pattern="Office",
                publisher_pattern="Microsoft%",
            )
            is True
        )
        assert (
            matches(
                "Office",
                "LibreOffice Team",
                "1.0",
                name_pattern="Office",
                publisher_pattern="Microsoft%",
            )
            is False
        )

    def test_null_publisher_against_publisher_pattern(self):
        # publisher unknown but pattern requires one → no match
        assert (
            matches("Office", None, "1.0", name_pattern="Office", publisher_pattern="Microsoft%")
            is False
        )

    def test_version_constraint_combined(self):
        assert (
            matches("Office", None, "2.5", name_pattern="Office", version_constraint=">=2.0,<3.0")
            is True
        )
        assert (
            matches("Office", None, "3.5", name_pattern="Office", version_constraint=">=2.0,<3.0")
            is False
        )
