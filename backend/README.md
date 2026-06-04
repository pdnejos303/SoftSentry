# SoftSentry Backend

FastAPI 0.115+ • SQLAlchemy 2.x async • Alembic • PostgreSQL 16 • Redis 7

## Quick start

### Local, no Docker (fastest — SQLite + in-memory Redis)

Self-contained run for clicking around `/docs` and exercising the API without
Postgres/Redis. Uses the committed `.venv`; schema built from the models, admin
auto-seeded from the project-root `.env`.

```powershell
.\run-local.ps1            # http://127.0.0.1:8000/docs  (login admin@local)
.\run-local.ps1 -Port 8001 -Reset
```

> Dev convenience only (single-process, non-persistent Redis shim). For real
> Postgres/Redis use the Docker path below. See `scripts/run_local.py`.

### Full stack (Docker — Postgres + Redis + worker)

```bash
docker compose up -d        # from repo root; migrations + seed run on boot
```

### Manual (uv, when installed)

```bash
uv sync
uv run alembic upgrade head
uv run python -m app.seed       # creates initial admin user
uv run uvicorn app.main:app --reload --port 8000
```

API docs (dev): http://localhost:8000/docs

## Commands

```bash
uv run pytest                                # tests
uv run ruff check . && uv run ruff format .  # lint + format
uv run mypy app                              # type check
uv run alembic revision -m "..."             # new migration
uv run alembic upgrade head                  # apply
uv run alembic downgrade -1                  # rollback
```

โครงสร้างละเอียดและ convention ดู `docs/09-coding-conventions.md` ที่ root repo
