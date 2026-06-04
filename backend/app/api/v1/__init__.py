"""v1 API router aggregation."""

from __future__ import annotations

from fastapi import APIRouter

from app.api.v1 import (
    agents,
    alerts,
    audit_logs,
    auth,
    dashboard,
    deploy,
    exports,
    licenses,
    machines,
    policy,
    reports,
    signatures,
    software,
    users,
    vulnerabilities,
)

api_router = APIRouter(prefix="/api/v1")
# Inline CSV exports first: their literal `/.../export` paths must be matched
# before the `/{uuid}` routes on machines/software/... routers (Module 8).
api_router.include_router(exports.router, tags=["exports"])
api_router.include_router(auth.router, prefix="/auth", tags=["auth"])
api_router.include_router(agents.router, prefix="/agents", tags=["agents"])
api_router.include_router(deploy.router, prefix="/deploy", tags=["deploy"])
api_router.include_router(machines.router, prefix="/machines", tags=["machines"])
api_router.include_router(software.router, prefix="/software", tags=["software"])
api_router.include_router(policy.whitelist_router, prefix="/whitelist", tags=["policy"])
api_router.include_router(policy.blacklist_router, prefix="/blacklist", tags=["policy"])
api_router.include_router(policy.policy_router, prefix="/policy", tags=["policy"])
api_router.include_router(alerts.router, prefix="/alerts", tags=["alerts"])
api_router.include_router(signatures.router, prefix="/signatures", tags=["signatures"])
api_router.include_router(signatures.dashboard_router, prefix="/dashboard", tags=["dashboard"])
api_router.include_router(
    vulnerabilities.router, prefix="/vulnerabilities", tags=["vulnerabilities"]
)
api_router.include_router(vulnerabilities.cve_router, prefix="/cve", tags=["vulnerabilities"])
api_router.include_router(vulnerabilities.dashboard_router, prefix="/dashboard", tags=["dashboard"])
api_router.include_router(vulnerabilities.admin_router, prefix="/admin", tags=["admin"])
api_router.include_router(licenses.router, prefix="/licenses", tags=["licenses"])
api_router.include_router(dashboard.router, prefix="/dashboard", tags=["dashboard"])
api_router.include_router(reports.router, prefix="/reports", tags=["reports"])
api_router.include_router(users.router, prefix="/users", tags=["users"])
api_router.include_router(audit_logs.router, prefix="/audit-logs", tags=["audit"])
