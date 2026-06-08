"""v1 API router aggregation."""

from __future__ import annotations

from fastapi import APIRouter, Depends

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
from app.core.deps import DEV, DEV_VIEWER, require_role


def _gate(*roles: str) -> list:
    """Router-level view gate — every route on the router requires one of `roles`.

    This is the *view* layer (who may see a module at all). Per-endpoint
    `require_role(...)` gates inside the routers remain the *mutation* layer, and
    both compose: e.g. a viewer passes a DEV_VIEWER view gate but still fails a
    DEV-only mutation gate. Kept in sync with dashboard/lib/permissions.ts.
    """
    return [Depends(require_role(*roles))]


api_router = APIRouter(prefix="/api/v1")
# Inline CSV exports first: their literal `/.../export` paths must be matched
# before the `/{uuid}` routes on machines/software/... routers (Module 8).
api_router.include_router(exports.router, tags=["exports"])
api_router.include_router(auth.router, prefix="/auth", tags=["auth"])
# Agents authenticate with agent tokens (not user roles); the one user-gated
# endpoint (enrollment-token) carries its own dependency.
api_router.include_router(agents.router, prefix="/agents", tags=["agents"])
# Deploy mixes user-gated endpoints (token minting, via the AdminUser dep =
# dev+admin) with the *installer download*, which authenticates by deployment
# token and no user login — so no router-level role gate here.
api_router.include_router(deploy.router, prefix="/deploy", tags=["deploy"])
# Machines are visible to every role; mutations are gated per-endpoint (dev+admin).
api_router.include_router(machines.router, prefix="/machines", tags=["machines"])
api_router.include_router(
    software.router, prefix="/software", tags=["software"], dependencies=_gate(*DEV_VIEWER)
)
api_router.include_router(
    policy.whitelist_router, prefix="/whitelist", tags=["policy"], dependencies=_gate(*DEV_VIEWER)
)
api_router.include_router(
    policy.blacklist_router, prefix="/blacklist", tags=["policy"], dependencies=_gate(*DEV_VIEWER)
)
api_router.include_router(
    policy.policy_router, prefix="/policy", tags=["policy"], dependencies=_gate(*DEV_VIEWER)
)
api_router.include_router(
    alerts.router, prefix="/alerts", tags=["alerts"], dependencies=_gate(*DEV_VIEWER)
)
api_router.include_router(
    signatures.router, prefix="/signatures", tags=["signatures"], dependencies=_gate(*DEV_VIEWER)
)
# Dashboard sub-routers feed the Overview, which every role (incl. admin) sees —
# so they stay open to all authenticated users.
api_router.include_router(signatures.dashboard_router, prefix="/dashboard", tags=["dashboard"])
api_router.include_router(
    vulnerabilities.router,
    prefix="/vulnerabilities",
    tags=["vulnerabilities"],
    dependencies=_gate(*DEV_VIEWER),
)
api_router.include_router(
    vulnerabilities.cve_router, prefix="/cve", tags=["vulnerabilities"], dependencies=_gate(*DEV_VIEWER)
)
api_router.include_router(vulnerabilities.dashboard_router, prefix="/dashboard", tags=["dashboard"])
api_router.include_router(
    vulnerabilities.admin_router, prefix="/admin", tags=["admin"], dependencies=_gate(*DEV)
)
api_router.include_router(
    licenses.router, prefix="/licenses", tags=["licenses"], dependencies=_gate(*DEV_VIEWER)
)
api_router.include_router(dashboard.router, prefix="/dashboard", tags=["dashboard"])
api_router.include_router(
    reports.router, prefix="/reports", tags=["reports"], dependencies=_gate(*DEV_VIEWER)
)
api_router.include_router(users.router, prefix="/users", tags=["users"], dependencies=_gate(*DEV))
api_router.include_router(
    audit_logs.router, prefix="/audit-logs", tags=["audit"], dependencies=_gate(*DEV)
)
