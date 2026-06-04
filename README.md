# SoftSentry

> Software Asset Management & Security tool สำหรับองค์กร — track ทุก software ที่ติดตั้งบนเครื่อง endpoint, ตรวจ digital signature, match CVE, จัดการ license compliance

[![Status](https://img.shields.io/badge/status-phases%201--5%20code--complete-brightgreen)]()
[![License](https://img.shields.io/badge/license-proprietary-blue)]()

---

## What it does

ติดตั้ง **agent** บนเครื่องพนักงาน (Windows / macOS) → agent สแกน software inventory และ digital signature → ส่งกลับ **central backend** → backend match กับ CVE database, ตรวจ license, แจ้งเตือน → IT admin ดู **dashboard** กลาง

### 9 Modules

1. **Agent** — Auto-scan + manual trigger, signature reading, heartbeat
2. **Software Inventory** — รายการ software ทุกเครื่อง, search/filter, history
3. **Digital Signature Verification** — Signed/Unsigned/Expired/Invalid + certificate chain
4. **Unauthorized Software Detection** — Whitelist/Blacklist + alert
5. **Vulnerability Check** — Match กับ NVD/OSV, แสดง CVE + severity
6. **License Compliance** — Track license, over-licensed flag, expiry alert
7. **Dashboard** — Risk score, charts, real-time feed
8. **Reporting & Export** — PDF/CSV, scheduled reports
9. **User & Access Management** — RBAC (Admin/Viewer), audit log

---

## Tech Stack

```
Agent (Go) ──HTTPS──▶ FastAPI (Python) ──▶ PostgreSQL
                          │
                          ├──▶ NVD/OSV API (CVE sync)
                          ▼
                  Next.js Dashboard
                  (shadcn/ui + Tailwind)
                          │
                          ▼
                  Prometheus + Grafana
```

| Layer | Stack |
|-------|-------|
| Agent | Go 1.22+ |
| Backend | Python 3.12 + FastAPI + SQLAlchemy 2 + Alembic |
| Frontend | Next.js 14 (App Router) + TypeScript + shadcn/ui |
| DB | PostgreSQL 16 |
| Observability | Prometheus + Grafana |
| i18n | next-intl (ไทย/EN) |

---

## Documentation

ทุก spec อยู่ใน `docs/`:

| ไฟล์ | เนื้อหา |
|------|---------|
| [`docs/00-overview.md`](docs/00-overview.md) | Product overview, users, scope |
| [`docs/01-tech-stack.md`](docs/01-tech-stack.md) | Tech choices + เหตุผล |
| [`docs/02-architecture.md`](docs/02-architecture.md) | System diagram, component boundaries |
| [`docs/03-data-model.md`](docs/03-data-model.md) | DB tables + relationships |
| [`docs/04-api-contracts.md`](docs/04-api-contracts.md) | REST endpoints |
| [`docs/05-agent-protocol.md`](docs/05-agent-protocol.md) | Agent↔Server protocol |
| [`docs/06-security.md`](docs/06-security.md) | Auth, enrollment, secrets |
| [`docs/07-dev-setup.md`](docs/07-dev-setup.md) | Local dev environment |
| [`docs/08-deployment.md`](docs/08-deployment.md) | Production deploy |
| [`docs/09-coding-conventions.md`](docs/09-coding-conventions.md) | Go/Python/TS style |
| [`docs/10-testing-strategy.md`](docs/10-testing-strategy.md) | Test pyramid |
| [`docs/modules/`](docs/modules/) | Spec ของ 9 modules |

**สำหรับ Claude Code:** อ่าน [`CLAUDE.md`](CLAUDE.md) ก่อน

---

## Roadmap

ดู [`ROADMAP.md`](ROADMAP.md) — แบ่งเป็น 5 phase

- **Phase 1**: Infra + auth backbone
- **Phase 2**: Agent + inventory (modules 1-2)
- **Phase 3**: Signature + Unauthorized + CVE (modules 3-5)
- **Phase 4**: License + Dashboard + Report (modules 6-8)
- **Phase 5**: User Mgmt + i18n + Telemetry + polish (module 9 + cross-cutting)

---

## Quick Start

> วิธีรันแบบละเอียด (URL, login, troubleshoot) ดู [`docs/RUN.md`](docs/RUN.md)

```bash
# รันทั้งระบบด้วย Docker คำสั่งเดียว (backend + worker + dashboard + postgres + redis)
git clone <repo> softsentry
cd softsentry
cp .env.example .env          # แล้วเติม secret จริง (openssl rand -hex 32)
docker compose up -d --build
# → Dashboard http://localhost:3000 · API http://localhost:8000 (login: admin@local / ChangeMe!2026)
# backend จะ migrate + seed admin ให้อัตโนมัติตอน boot

# Backend dev
cd backend && uv sync && uv run uvicorn app.main:app --reload

# Dashboard dev
cd dashboard && pnpm install && pnpm dev

# Agent build (Windows)
cd agent && GOOS=windows GOARCH=amd64 go build -o dist/softsentry-agent.exe ./cmd/softsentry-agent

# Agent build (macOS)
cd agent && GOOS=darwin GOARCH=arm64 go build -o dist/softsentry-agent ./cmd/softsentry-agent
```

---

## Status

🟢 **Phases 1–5 — code-complete** (modules 1–9 + i18n + telemetry + polish)

- **Backend** (FastAPI + SQLAlchemy + Alembic) — auth, agent enrollment, inventory, signature, unauthorized, CVE, license, dashboard, reporting, user management + **audit log on every mutation** (license/policy/alert/vuln/report/cve-sync), **Prometheus `/metrics`** (HTTP + agent + business KPIs)
  - ✅ `ruff` สะอาด, `mypy --strict` ผ่าน (94 files), **270 pytest ผ่าน** (SQLite + in-memory Redis)
- **Agent** (Go + cobra CLI) — scanner (Win/Mac), signature verify, heartbeat/scan, auto-update, service install; `go build`/`vet`/`test` ผ่าน
- **Dashboard** (Next.js 14 + shadcn/ui + Tailwind + next-intl) — ทุกหน้า module 1–9, **language switcher (ไทย/EN)**, error/404 + loading skeletons
  - ✅ `lint`/`typecheck` สะอาด, **57 vitest ผ่าน**, `next build` compiles (standalone symlink เป็น Windows-only env limit; OK ใน Docker)
- **Observability** — Prometheus scrape + 3 provisioned Grafana dashboards (backend health / agent fleet / business KPIs) ใน `infra/grafana/`
- CI — `.github/workflows/ci.yml` (lint + test ทั้ง 3 service)

**Pending live runs** (code done): Win/Mac agent install (needs admin), full CVE-feed sync timing, PDF render (WeasyPrint needs Linux), email delivery, `docker compose --profile observability up` scrape check, a11y/Lighthouse audits.

ดูรายละเอียดแต่ละ phase ใน [`ROADMAP.md`](ROADMAP.md)
"# SoftSentry" 
