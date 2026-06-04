"""Pure pattern-matching engine for whitelist/blacklist policy (Module 4).

Mirrors SQL ILIKE semantics for name/publisher patterns and uses
`packaging.SpecifierSet` for version constraints, so the background matcher
behaves the same whether it runs in Python or (later) pushes down to the DB.
"""

from __future__ import annotations

import re

from packaging.specifiers import InvalidSpecifier, SpecifierSet
from packaging.version import InvalidVersion, Version


def _like_to_regex(pattern: str) -> re.Pattern[str]:
    """Translate an SQL LIKE pattern to an anchored, case-insensitive regex.

    `%` → 0+ chars, `_` → exactly one char; every other character is literal.
    """
    parts = ["^"]
    for ch in pattern:
        if ch == "%":
            parts.append(".*")
        elif ch == "_":
            parts.append(".")
        else:
            parts.append(re.escape(ch))
    parts.append("$")
    return re.compile("".join(parts), re.IGNORECASE | re.DOTALL)


def ilike_match(value: str, pattern: str) -> bool:
    """SQL ILIKE: `%`/`_` wildcards, case-insensitive, whole-string match.

    A pattern with no wildcards therefore behaves as a case-insensitive exact
    match.
    """
    return _like_to_regex(pattern).match(value or "") is not None


def version_matches(version: str, constraint: str | None) -> bool:
    """True if `version` satisfies `constraint` (a `SpecifierSet` string).

    Best-effort per spec: an empty/absent constraint always matches, and a
    version or constraint that can't be parsed matches by default (so a policy
    entry never silently drops non-semver software).
    """
    if not constraint:
        return True
    try:
        spec = SpecifierSet(constraint)
    except InvalidSpecifier:
        return True
    try:
        return Version(version) in spec
    except InvalidVersion:
        return True


def matches(
    name: str,
    publisher: str | None,
    version: str,
    *,
    name_pattern: str,
    publisher_pattern: str | None = None,
    version_constraint: str | None = None,
) -> bool:
    """True if a software record matches a policy entry (spec 4 §Pattern matching).

    name must match; publisher_pattern (when set) must match the record's
    publisher — an unknown publisher never matches a publisher_pattern;
    version_constraint (when set) must be satisfied.
    """
    if not ilike_match(name, name_pattern):
        return False
    if publisher_pattern and not ilike_match(publisher or "", publisher_pattern):
        return False
    return version_matches(version, version_constraint)
