"""Overview-dashboard endpoints (Module 7) — KPIs, top-N risk, vuln trend.

Signature-status and license-compliance charts are served by the existing
`/dashboard/signature-stats` and `/licenses/compliance-summary` endpoints, which
the overview page reuses.
"""

from __future__ import annotations

from typing import Annotated

from fastapi import APIRouter, Query

from app.core.deps import CurrentUser, DBSession
from app.schemas.dashboard import Overview, RiskScoreItem, RiskScoreList, VulnTrend, VulnTrendPoint
from app.services import dashboard_service

router = APIRouter()

_PERIODS = {"7d": 7, "30d": 30, "90d": 90}


@router.get("/overview", response_model=Overview, summary="Overview KPI counts")
async def overview(session: DBSession, _: CurrentUser) -> Overview:
    return Overview(**await dashboard_service.overview(session))


@router.get("/risk-scores", response_model=RiskScoreList, summary="Top-N risky machines")
async def risk_scores(
    session: DBSession,
    _: CurrentUser,
    limit: Annotated[int, Query(ge=1, le=50)] = 10,
) -> RiskScoreList:
    items = await dashboard_service.risk_scores(session, limit=limit)
    return RiskScoreList(items=[RiskScoreItem(**i) for i in items])


@router.get(
    "/charts/vuln-trend",
    response_model=VulnTrend,
    summary="Daily vulnerability counts by severity",
)
async def vuln_trend(
    session: DBSession,
    _: CurrentUser,
    period: str = "30d",
) -> VulnTrend:
    days = _PERIODS.get(period, 30)
    points = await dashboard_service.vuln_trend(session, days=days)
    return VulnTrend(
        period_days=days,
        points=[VulnTrendPoint(**p) for p in points],
    )
