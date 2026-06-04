# SoftSentry — Claude Code Guide

> ไฟล์นี้ Claude Code โหลดอัตโนมัติทุก session — เก็บเฉพาะ context สำคัญที่ต้องรู้ก่อนแตะโค้ด

## Project ที่กำลังทำ

**SoftSentry** — Software Asset Management + Security tool สำหรับองค์กร

ติดตั้ง agent บนเครื่อง endpoint (Windows/Mac) → agent สแกน software inventory + digital signature → ส่งกลับ backend → backend match กับ CVE database (NVD/OSV), จัดการ license, แจ้งเตือน → IT admin ดู dashboard

**สถานะ:** Greenfield. ยังไม่มี code. ทุก doc ใน `docs/` เป็น spec ที่ใช้ตอน implement

---

## Tech Stack

| Layer | Stack | หมายเหตุ |
|-------|-------|----------|
| Agent | **Go 1.22+** | Single binary, cross-compile Win/Mac, ใช้ `golang.org/x/sys/windows/registry`, Authenticode verify |
| Backend | **Python 3.12 + FastAPI** | Async, auto OpenAPI, SQLAlchemy 2.x + Alembic |
| Frontend | **Next.js 14 (App Router) + TypeScript + shadcn/ui + Tailwind** | React Query, Recharts |
| DB | **PostgreSQL 16** | Run ใน Docker |
| Infra | **Docker Compose** dev + prod | Nginx reverse proxy |
| Observability | **Prometheus + Grafana** | metrics จากทั้ง agent + backend |
| i18n | **next-intl** (frontend) | รองรับ ไทย/EN |

อ่านรายละเอียดเหตุผลการเลือกที่ [`docs/01-tech-stack.md`](docs/01-tech-stack.md)

---

## Directory Layout (ที่จะสร้าง)

```
SoftSentry/
├── agent/                 # Go source — single binary
│   ├── cmd/
│   ├── internal/
│   │   ├── scanner/       # Software inventory scan
│   │   ├── signature/     # Authenticode/Codesign verify
│   │   ├── transport/     # HTTP client → backend
│   │   └── service/       # Windows Service / launchd wrapper
│   └── go.mod
├── backend/               # FastAPI
│   ├── app/
│   │   ├── api/v1/        # Routers แยกตาม module
│   │   ├── core/          # config, security, db
│   │   ├── models/        # SQLAlchemy models
│   │   ├── schemas/       # Pydantic schemas
│   │   ├── services/      # Business logic
│   │   └── workers/       # Background tasks (CVE sync, etc)
│   ├── alembic/
│   ├── tests/
│   └── pyproject.toml
├── dashboard/             # Next.js
│   ├── app/
│   │   ├── (auth)/
│   │   ├── (dashboard)/
│   │   └── api/
│   ├── components/
│   ├── lib/
│   └── package.json
├── docs/                  # Spec ทั้งหมด (อ่านก่อนเขียน code)
└── docker-compose.yml
```

---

## หลักการที่ต้องยึด (Non-negotiable)

1. **อ่าน spec ก่อนเขียน code** — ทุก module มี spec อยู่ใน `docs/modules/0X-*.md` พร้อม acceptance criteria
2. **TDD เมื่อเป็นไปได้** — เขียน test ก่อน implementation โดยเฉพาะ business logic (vulnerability matching, license calculation)
3. **Agent ต้องเป็น single binary** — ห้าม depend on runtime/interpreter ที่ผู้ใช้ต้องลงเพิ่ม
4. **Multi-platform from day 1** — code agent ต้องรองรับทั้ง Windows และ macOS ตั้งแต่แรก
5. **Security by default** — ทุก API ผ่าน auth, agent enrollment ใช้ token, secrets ไม่อยู่ใน code
6. **i18n-aware** — ทุก user-facing string ใน dashboard ผ่าน next-intl (ห้าม hardcode ภาษา)

---

## ลำดับการ implement

ดู [`ROADMAP.md`](ROADMAP.md) — แบ่งเป็น 5 phase ตั้งแต่ infra → agent → core modules → security modules → polish

**ห้าม implement หลาย module พร้อมกัน** จนกว่า phase ก่อนหน้าเสร็จและ test ผ่าน

---

## เอกสารที่ต้องอ่านก่อนแตะแต่ละ layer

| ก่อนเขียน... | อ่าน |
|--------------|------|
| Agent code | `docs/modules/01-agent.md`, `docs/05-agent-protocol.md`, `docs/06-security.md` |
| Backend API | `docs/04-api-contracts.md`, `docs/03-data-model.md`, `docs/06-security.md` |
| Dashboard | `docs/modules/07-dashboard.md`, `docs/09-coding-conventions.md` |
| Module ใดๆ | `docs/modules/0X-*.md` ของ module นั้นๆ |
| Test | `docs/10-testing-strategy.md` |

---

## Convention สรุปสั้นๆ (รายละเอียดใน docs/09)

- **Go**: `gofmt`, `golangci-lint`, errors wrap ด้วย `fmt.Errorf("...: %w", err)`, ไม่ใช้ panic ใน production path
- **Python**: `ruff` + `black`, type hints บังคับ, `async def` สำหรับ I/O ทุกตัว, Pydantic v2 models
- **TypeScript**: `eslint` + `prettier`, `strict: true`, ห้าม `any`, React Server Components by default

---

## Build/Test Commands (จะถูก setup ภายหลัง)

```bash
# Agent
cd agent && go build ./cmd/softsentry-agent
cd agent && go test ./...

# Backend
cd backend && uv run uvicorn app.main:app --reload
cd backend && uv run pytest

# Dashboard
cd dashboard && pnpm dev
cd dashboard && pnpm test

# All-in-one dev environment
docker compose up -d
```

---

## เมื่อไม่แน่ใจ

- spec ไม่ชัด → **ถามผู้ใช้** ก่อนเดา. อย่า invent feature
- conflict ระหว่าง modules → อ่าน `docs/02-architecture.md` ตัดสิน
- security decision → default ไปทางปลอดภัยกว่า, ถามถ้าจะ trade-off
