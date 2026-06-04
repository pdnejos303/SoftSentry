"""Dev-only launcher: run the backend on a port with ZERO external services.

Why this exists
---------------
The production/Docker path uses PostgreSQL + Redis (see docker-compose.yml).
On a bare dev machine without Docker running you still want to click around
/docs, log in, and hit the API. This script wires up a self-contained stack:

  * SQLite file DB (schema built from the SQLAlchemy models via ``create_all``
    — NOT alembic, because the initial migration hardcodes Postgres ``JSONB``).
  * An in-memory Redis shim (only the handful of ops auth_service uses).

It does NOT touch any production code — the Redis fake lives here and is
injected at runtime. For the real stack use ``docker compose up``.

Usage (from the backend/ dir, using the committed venv):
    .venv\\Scripts\\python.exe -m scripts.run_local
    .venv\\Scripts\\python.exe -m scripts.run_local --port 8001 --reset

Then open http://localhost:8000/docs and log in with the seeded admin
(INITIAL_ADMIN_EMAIL / INITIAL_ADMIN_PASSWORD from the project-root .env).
"""

from __future__ import annotations

import argparse
import asyncio
import os
import time
from pathlib import Path

BACKEND_DIR = Path(__file__).resolve().parent.parent
ROOT_ENV = BACKEND_DIR.parent / ".env"
DB_FILE = BACKEND_DIR / "softsentry_local.db"


def _load_root_env() -> None:
    """Seed os.environ from the project-root .env (Docker passes this via
    ``env_file``; running locally we have to load it ourselves). Existing env
    vars win, so CLI/shell overrides still take precedence."""
    if not ROOT_ENV.exists():
        return
    for raw in ROOT_ENV.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, _, value = line.partition("=")
        key = key.strip()
        # Strip inline "  # comment" (our secrets are hex/URLs, never contain #).
        value = value.split(" #", 1)[0].strip().strip('"').strip("'")
        os.environ.setdefault(key, value)


def _configure_env(reset: bool) -> None:
    _load_root_env()
    # Force the self-contained backends regardless of what .env said.
    if reset and DB_FILE.exists():
        DB_FILE.unlink()
    os.environ["DATABASE_URL"] = f"sqlite+aiosqlite:///{DB_FILE.as_posix()}"
    os.environ["REDIS_URL"] = "redis://localhost:6379/0"  # never dialed; shim injected
    os.environ.setdefault("ENV", "development")
    os.environ.setdefault("JWT_SECRET", "dev_local_insecure_secret_change_me")


class _InMemoryRedis:
    """Minimal async Redis substitute — implements exactly the ops
    auth_service uses (get/set/delete/incr/expire/ttl). Single-process,
    non-persistent: fine for local dev, useless for anything else."""

    def __init__(self) -> None:
        self._store: dict[str, str] = {}
        self._expires: dict[str, float] = {}

    def _expired(self, key: str) -> bool:
        exp = self._expires.get(key)
        if exp is not None and exp <= time.time():
            self._store.pop(key, None)
            self._expires.pop(key, None)
            return True
        return False

    async def get(self, key: str) -> str | None:
        if self._expired(key):
            return None
        return self._store.get(key)

    async def set(self, key: str, value: str, ex: int | None = None) -> None:
        self._store[key] = str(value)
        if ex:
            self._expires[key] = time.time() + ex

    async def delete(self, *keys: str) -> int:
        n = 0
        for k in keys:
            if self._store.pop(k, None) is not None:
                n += 1
            self._expires.pop(k, None)
        return n

    async def incr(self, key: str) -> int:
        self._expired(key)
        new = int(self._store.get(key, "0")) + 1
        self._store[key] = str(new)
        return new

    async def expire(self, key: str, seconds: int) -> bool:
        if key in self._store and not self._expired(key):
            self._expires[key] = time.time() + seconds
            return True
        return False

    async def ttl(self, key: str) -> int:
        if self._expired(key) or key not in self._store:
            return -2
        exp = self._expires.get(key)
        if exp is None:
            return -1
        remaining = int(exp - time.time())
        return remaining if remaining > 0 else -2


async def _bootstrap() -> None:
    """Build the schema and seed the admin against the local SQLite file."""
    from app.core import redis as redis_module
    from app.core.db import engine
    from app.models import Base  # imports every model → metadata populated
    from app.seed import seed_admin

    redis_module._redis = _InMemoryRedis()  # type: ignore[assignment]

    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    await seed_admin()
    # Drop pooled connections bound to this throwaway loop; uvicorn's own loop
    # will open fresh ones (avoids "Future attached to a different loop").
    await engine.dispose()


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Run SoftSentry backend locally (SQLite + in-memory Redis)."
    )
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=8000)
    parser.add_argument("--reset", action="store_true", help="Delete the local SQLite DB first.")
    args = parser.parse_args()

    _configure_env(args.reset)

    # Import uvicorn only after env is set so Settings() reads our values.
    import uvicorn

    asyncio.run(_bootstrap())

    from app.core import redis as redis_module
    from app.main import app

    # Re-inject the shim: uvicorn.run creates a fresh event loop but the module
    # global persists across loops, so the same instance is reused. Ensure it's set.
    if redis_module._redis is None:  # pragma: no cover - defensive
        redis_module._redis = _InMemoryRedis()  # type: ignore[assignment]

    print("\n" + "=" * 60)
    print("  SoftSentry backend (LOCAL dev — SQLite + in-mem Redis)")
    print(f"  API   : http://{args.host}:{args.port}/api/v1")
    print(f"  Docs  : http://{args.host}:{args.port}/docs")
    print(f"  Health: http://{args.host}:{args.port}/health")
    print(
        f"  Admin : {os.environ.get('INITIAL_ADMIN_EMAIL', 'admin@local')}"
        f" / {os.environ.get('INITIAL_ADMIN_PASSWORD', 'ChangeMe!2026')}"
    )
    print("=" * 60 + "\n")

    uvicorn.run(app, host=args.host, port=args.port, log_level="info")


if __name__ == "__main__":
    main()
