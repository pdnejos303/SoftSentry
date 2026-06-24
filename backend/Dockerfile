# syntax=docker/dockerfile:1.7

FROM python:3.12-slim AS base
ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=1 \
    UV_LINK_MODE=copy \
    UV_PROJECT_ENVIRONMENT=/app/.venv

RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential libpq-dev curl \
      libpango-1.0-0 libpangoft2-1.0-0 libcairo2 fonts-thai-tlwg \
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir uv

WORKDIR /app

# Copy the lockfile too so `uv sync --frozen` installs the EXACT pinned versions
# instead of re-resolving the dependency graph from PyPI on every build (faster +
# reproducible). This deps layer is cached until pyproject.toml/uv.lock change.
COPY pyproject.toml uv.lock ./
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --frozen --no-dev --no-install-project

COPY app ./app
COPY alembic.ini ./alembic.ini
COPY alembic ./alembic
# Required because pyproject.toml declares `readme = "README.md"`; the project
# build (editable install below) fails without it.
COPY README.md ./README.md

RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync --frozen --no-dev

# Runtime hardening: every `uv run` (this image's CMD, plus the compose backend
# command and worker command) defaults to syncing the project environment first.
# Because the image is built with `--no-dev`, that pre-run sync re-resolves the
# lockfile, sees the dev group (ruff/mypy/pytest) is absent, and DOWNLOADS it at
# container startup — slow and network-dependent, and it stalls the whole boot if
# the network hiccups, so the backend never reaches uvicorn and login breaks.
# UV_NO_SYNC pins every `uv run` to the already-baked /app/.venv: no network, no
# dev tooling, deterministic startup. Set AFTER the build-time `uv sync` steps so
# they still install normally. Dev/test runs use the local `uv` workflow, not this
# production image.
ENV UV_NO_SYNC=1

EXPOSE 8000

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD curl -fsS http://localhost:8000/health || exit 1

CMD ["uv", "run", "uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
