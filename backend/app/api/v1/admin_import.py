"""Admin: trigger a MongoDB → PostgreSQL import on demand."""

from __future__ import annotations

from typing import Annotated

from fastapi import APIRouter, Depends, HTTPException, Request, status
from pydantic import BaseModel

from app.api.v1._audit import audit
from app.core.deps import DBSession, require_role
from app.models.user import User
from app.workers.mongo_import import enqueue_mongo_import

router = APIRouter()


class ImportTriggered(BaseModel):
    queued: bool


@router.post(
    "/mongo-import/trigger",
    response_model=ImportTriggered,
    status_code=status.HTTP_202_ACCEPTED,
    summary="Admin: trigger a MongoDB inventory import now",
)
async def trigger_mongo_import(
    session: DBSession,
    user: Annotated[User, Depends(require_role("admin"))],
    request: Request,
    full: bool = False,
) -> ImportTriggered:
    # full=True → ignore the lookback window (for first/full import); else use config default.
    queued = await enqueue_mongo_import(lookback_minutes=999_999 if full else None)
    if not queued:
        raise HTTPException(status.HTTP_503_SERVICE_UNAVAILABLE, "Job queue unavailable")
    await audit(
        session,
        request,
        user,
        action="mongo_import.trigger",
        entity_type="mongo_import",
        after={"full": full},
    )
    await session.commit()
    return ImportTriggered(queued=True)