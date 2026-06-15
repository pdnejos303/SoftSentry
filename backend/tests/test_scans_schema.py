"""Schema validation tests for scan ingestion source values."""

import pytest
from pydantic import ValidationError

from app.schemas.scans import SoftwareIn


def _base(**overrides):
    data = {"name": "App", "version": "1.0", "source": "registry"}
    data.update(overrides)
    return data


def test_software_in_accepts_filesystem_source():
    sw = SoftwareIn(**_base(source="filesystem"))
    assert sw.source == "filesystem"


def test_software_in_rejects_unknown_source():
    with pytest.raises(ValidationError):
        SoftwareIn(**_base(source="bogus"))
