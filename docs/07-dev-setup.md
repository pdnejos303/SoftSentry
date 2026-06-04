# 07 — Development Setup

> **แค่อยากรันให้ขึ้นเฉยๆ?** ไป [`RUN.md`](RUN.md) — `docker compose up -d --build` คำสั่งเดียวจบ
> ไฟล์นี้สำหรับ dev แบบลงมือแก้ทีละ service (รัน backend/dashboard/agent นอก container)

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Docker Desktop | latest | https://docker.com |
| Git | 2.40+ | system pkg |
| Go | 1.22+ | https://go.dev/dl |
| Python | 3.12+ | https://python.org |
| `uv` | latest | `pip install uv` หรือ `curl -LsSf https://astral.sh/uv/install.sh \| sh` |
| Node.js | 20 LTS | nvm หรือ https://nodejs.org |
| `pnpm` | 9+ | `npm i -g pnpm` |
| `mkcert` | latest | สำหรับ local TLS |

---

## First-time setup

```bash
git clone <repo> softsentry
cd softsentry
cp .env.example .env
# แก้ค่าใน .env ตามต้องการ — generate JWT_SECRET ด้วย `openssl rand -hex 32`

# Generate local TLS cert
mkcert -install
mkcert localhost 127.0.0.1
mv localhost+1.pem infra/nginx/certs/cert.pem
mv localhost+1-key.pem infra/nginx/certs/key.pem

# Bring up infra (postgres, redis, prometheus, grafana)
docker compose up -d postgres redis

# Backend
cd backend
uv sync
uv run alembic upgrade head
uv run python -m app.seed   # creates initial admin user
uv run uvicorn app.main:app --reload --port 8000
```

แยก terminal:
```bash
# Dashboard
cd dashboard
pnpm install
pnpm dev
# → http://localhost:3000
```

แยก terminal:
```bash
# Worker
cd backend
uv run arq app.workers.WorkerSettings
```

แยก terminal:
```bash
# Agent (local test)
cd agent
go run ./cmd/softsentry-agent enroll --token <enrollment-token-from-dashboard>
go run ./cmd/softsentry-agent run
```

---

## .env.example

```bash
# ───── Backend ─────
DATABASE_URL=postgresql+asyncpg://softsentry:dev_password@localhost:5432/softsentry
REDIS_URL=redis://:dev_password@localhost:6379/0
JWT_SECRET=                              # openssl rand -hex 32
JWT_ACCESS_TTL_SECONDS=3600
JWT_REFRESH_TTL_SECONDS=2592000
BCRYPT_PEPPER=                           # optional, blank ok in dev
LICENSE_KEY_ENCRYPTION_KEY=              # openssl rand -hex 32
DASHBOARD_URL=http://localhost:3000
LOG_LEVEL=INFO
ENV=development                          # development | staging | production
ENABLE_DOCS=true                         # Swagger /docs

# Initial admin (used by seed script)
INITIAL_ADMIN_EMAIL=admin@local
INITIAL_ADMIN_PASSWORD=ChangeMe!2026

# NVD/OSV
NVD_API_KEY=                             # optional but recommended (60 req/min vs 6)
OSV_API_BASE=https://api.osv.dev/v1

# ───── Dashboard ─────
NEXT_PUBLIC_API_URL=http://localhost:8000/api/v1
NEXT_PUBLIC_DEFAULT_LOCALE=th

# ───── Docker Compose ─────
POSTGRES_USER=softsentry
POSTGRES_PASSWORD=dev_password
POSTGRES_DB=softsentry
REDIS_PASSWORD=dev_password
```

---

## docker-compose.yml (sketch)

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: ${POSTGRES_DB}
    volumes:
      - pg_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER}"]
      interval: 5s

  redis:
    image: redis:7-alpine
    command: redis-server --requirepass ${REDIS_PASSWORD}
    ports:
      - "6379:6379"

  backend:
    build: ./backend
    depends_on: { postgres: { condition: service_healthy }, redis: { condition: service_started } }
    env_file: .env
    ports:
      - "8000:8000"
    develop:
      watch:
        - { action: sync+restart, path: ./backend/app, target: /app/app }

  worker:
    build: ./backend
    command: arq app.workers.WorkerSettings
    depends_on: [postgres, redis]
    env_file: .env

  dashboard:
    build: ./dashboard
    depends_on: [backend]
    env_file: .env
    ports:
      - "3000:3000"

  nginx:
    image: nginx:alpine
    volumes:
      - ./infra/nginx/nginx.conf:/etc/nginx/nginx.conf:ro
      - ./infra/nginx/certs:/etc/nginx/certs:ro
    ports:
      - "443:443"
      - "80:80"
    depends_on: [backend, dashboard]

  prometheus:
    image: prom/prometheus:latest
    volumes:
      - ./infra/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
    ports:
      - "9090:9090"

  grafana:
    image: grafana/grafana:latest
    volumes:
      - grafana_data:/var/lib/grafana
      - ./infra/grafana/dashboards:/etc/grafana/provisioning/dashboards:ro
    ports:
      - "3001:3000"
    environment:
      GF_SECURITY_ADMIN_PASSWORD: ${POSTGRES_PASSWORD}

volumes:
  pg_data:
  grafana_data:
```

---

## Common commands

```bash
# Backend
cd backend
uv run pytest                            # all tests
uv run pytest tests/api/test_machines.py # one file
uv run pytest -k "test_login"            # match name
uv run alembic revision -m "add ..."     # new migration
uv run alembic upgrade head              # apply
uv run alembic downgrade -1              # rollback 1
uv run ruff check . && uv run ruff format .
uv run mypy app

# Dashboard
cd dashboard
pnpm dev
pnpm test
pnpm test:e2e                            # playwright
pnpm lint
pnpm typecheck
pnpm build                               # production build

# Agent
cd agent
go test ./...
go test ./internal/scanner -run TestWindowsRegistry
golangci-lint run
go build -o dist/agent ./cmd/softsentry-agent
GOOS=windows GOARCH=amd64 go build -o dist/agent.exe ./cmd/softsentry-agent
GOOS=darwin GOARCH=arm64 go build -o dist/agent-mac ./cmd/softsentry-agent

# Docker
docker compose up -d
docker compose logs -f backend
docker compose down -v                   # nuke volumes too
docker compose restart backend
```

---

## Debugging

### Backend
- VSCode + Python extension: launch.json with `uvicorn app.main:app --reload`
- ใช้ `breakpoint()` หรือ `import pdb; pdb.set_trace()`
- Log SQL: set `LOG_LEVEL=DEBUG` → SQLAlchemy logs queries

### Dashboard
- VSCode + Next.js debugger
- React DevTools + React Query DevTools

### Agent
- `go run ./cmd/softsentry-agent --log-level debug run`
- Test scan แบบไม่ต้องลง service: `./agent scan --output stdout`

---

## Database access

```bash
# psql
docker compose exec postgres psql -U softsentry softsentry

# หรือ install pgcli: pip install pgcli
pgcli postgresql://softsentry:dev_password@localhost:5432/softsentry
```

---

## Reset everything

```bash
docker compose down -v       # delete volumes
rm -rf backend/.venv dashboard/node_modules agent/dist
# Then redo first-time setup
```

---

## Pre-commit hooks (recommended)

`.pre-commit-config.yaml`:
```yaml
repos:
  - repo: https://github.com/astral-sh/ruff-pre-commit
    rev: v0.5.0
    hooks: [{id: ruff}, {id: ruff-format}]
  - repo: https://github.com/golangci/golangci-lint
    rev: v1.59.0
    hooks: [{id: golangci-lint, files: ^agent/}]
  - repo: local
    hooks:
      - id: pnpm-lint
        name: pnpm lint
        entry: bash -c 'cd dashboard && pnpm lint --quiet'
        language: system
        files: ^dashboard/
        pass_filenames: false
```

Install: `pip install pre-commit && pre-commit install`
