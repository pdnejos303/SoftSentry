"""Module 7 — pure risk-score formula + color-threshold mapping."""

from __future__ import annotations

from app.services.risk_service import compute_risk_score, risk_color


class TestComputeRiskScore:
    def test_no_issues_is_zero(self):
        assert (
            compute_risk_score(
                unsigned=0,
                unauthorized=0,
                cve_critical=0,
                cve_high=0,
                cve_medium=0,
                cve_low=0,
            )
            == 0
        )

    def test_single_critical_cve_is_five(self):
        assert (
            compute_risk_score(
                unsigned=0,
                unauthorized=0,
                cve_critical=1,
                cve_high=0,
                cve_medium=0,
                cve_low=0,
            )
            == 5
        )

    def test_weighted_sum(self):
        # 10 unauthorized (x3 = 30) + 2 critical (x5 = 10) = 40 (spec acceptance).
        assert (
            compute_risk_score(
                unsigned=0,
                unauthorized=10,
                cve_critical=2,
                cve_high=0,
                cve_medium=0,
                cve_low=0,
            )
            == 40
        )

    def test_all_signals(self):
        # 2*1 + 1*3 + 1*5 + 2*3 + 3*1 + 4*0.5 = 2 + 3 + 5 + 6 + 3 + 2 = 21
        assert (
            compute_risk_score(
                unsigned=2,
                unauthorized=1,
                cve_critical=1,
                cve_high=2,
                cve_medium=3,
                cve_low=4,
            )
            == 21
        )

    def test_low_contributes_half(self):
        assert (
            compute_risk_score(
                unsigned=0,
                unauthorized=0,
                cve_critical=0,
                cve_high=0,
                cve_medium=0,
                cve_low=1,
            )
            == 0.5
        )


class TestRiskColor:
    def test_zero_is_green(self):
        assert risk_color(0) == "green"

    def test_low_band_yellow(self):
        assert risk_color(1) == "yellow"
        assert risk_color(5) == "yellow"
        assert risk_color(10) == "yellow"

    def test_mid_band_orange(self):
        assert risk_color(11) == "orange"
        assert risk_color(30) == "orange"

    def test_high_band_red(self):
        assert risk_color(31) == "red"
        assert risk_color(40) == "red"
        assert risk_color(999) == "red"
