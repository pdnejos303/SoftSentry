# 01 — Tech Stack & Rationale

## สรุปการเลือก

| Layer | Choice | ทางเลือกที่พิจารณา | เหตุผลที่ไม่เลือก |
|-------|--------|---------------------|-------------------|
| Agent | **Go 1.22+** | C# / Rust / Python | C# ต้องลง .NET runtime, Rust learning curve สูง, Python bundle ใหญ่ |
| Backend | **Python + FastAPI** | Node.js, Go, Java Spring | Claude เขียน Python ได้ดีสุด, security/CVE library เยอะ |
| Frontend | **Next.js + shadcn/ui** | Vite+React, SvelteKit, Remix | Ecosystem โต, SSR ฟรี, component library สมบูรณ์ |
| DB | **PostgreSQL 16** | MySQL, SQLite, MongoDB | JSON support, mature, scale ดี |

---

## Agent — Go 1.22+

### เหตุผล
- **Single binary** — `go build` ออกมาเป็นไฟล์เดียว ไม่ต้องลง runtime
- **Cross-compile ง่าย** — Windows/Mac จากเครื่อง dev เครื่องเดียว
- **Memory footprint ต่ำ** — เหมาะรัน background service ตลอด
- **Concurrency model ดี** — goroutine สำหรับ scan parallel ง่ายมาก

### Libraries ที่จะใช้

| ใช้ทำ | Library |
|--------|---------|
| Windows Registry | `golang.org/x/sys/windows/registry` |
| Windows Service | `golang.org/x/sys/windows/svc` |
| Authenticode parse | `github.com/saferwall/pe` |
| macOS plist | `howett.net/plist` |
| macOS codesign | shell out `codesign --verify` |
| HTTP client | stdlib `net/http` + retry: `github.com/hashicorp/go-retryablehttp` |
| Config | `github.com/spf13/viper` |
| CLI | `github.com/spf13/cobra` |
| Logging | `log/slog` (stdlib, Go 1.21+) |
| Cron scheduler | `github.com/robfig/cron/v3` |

### ไม่ใช้
- ❌ CGO — ทำให้ cross-compile ลำบาก
- ❌ External daemon (electron, node) — ทำลายจุดเด่น single binary

---

## Backend — Python 3.12 + FastAPI

### เหตุผล
- **Claude เขียน Python ได้ดีที่สุด** — ลด churn
- **FastAPI** — async + auto OpenAPI doc + Pydantic validation ฟรี
- **Security library เยอะ** — `python-jose`, `passlib`, `cryptography`
- **PDF/report generation ดี** — `weasyprint`, `reportlab`

### Libraries ที่จะใช้

| ใช้ทำ | Library |
|--------|---------|
| Web framework | `fastapi` |
| ASGI server | `uvicorn` (dev), `gunicorn` + `uvicorn workers` (prod) |
| ORM | `sqlalchemy` 2.x (async) |
| Migration | `alembic` |
| Validation | `pydantic` v2 |
| Auth | `python-jose[cryptography]` (JWT), `passlib[bcrypt]` |
| HTTP client | `httpx` (async) — สำหรับเรียก NVD/OSV |
| Background tasks | `arq` (Redis-based) หรือ FastAPI BackgroundTasks สำหรับงานสั้น |
| PDF | `weasyprint` |
| CSV | stdlib `csv` |
| CVE parsing | `cve-parse` หรือ parse NVD JSON เอง |
| Test | `pytest`, `pytest-asyncio`, `httpx` (client) |
| Lint/Format | `ruff`, `black` |
| Type check | `mypy` strict |

### Package manager
- **`uv`** — เร็วกว่า pip 10-100x, lockfile ดี
- `pyproject.toml` only, ไม่ใช้ `requirements.txt`

### Structure
```
backend/app/
├── main.py              # FastAPI app
├── core/
│   ├── config.py        # Settings (pydantic-settings)
│   ├── security.py      # JWT, password hashing
│   └── db.py            # SQLAlchemy session
├── models/              # SQLAlchemy ORM models
├── schemas/             # Pydantic request/response
├── api/v1/              # Routers — 1 file per module
├── services/            # Business logic (pure functions)
└── workers/             # Background: cve_sync, license_expiry_check
```

---

## Frontend — Next.js 14 + shadcn/ui

### เหตุผล
- **App Router + RSC** — fetch ใน server, ลด client bundle
- **shadcn/ui** — copy-paste components, owned ใน repo, customizable เต็มที่
- **Tailwind** — ไม่ต้องคิดเรื่อง CSS architecture
- **Built-in i18n** ผ่าน next-intl + App Router

### Libraries ที่จะใช้

| ใช้ทำ | Library |
|--------|---------|
| Framework | `next` 14+ (App Router) |
| Language | `typescript` (strict) |
| UI primitives | `shadcn/ui` (Radix-based) |
| Styling | `tailwindcss` |
| State (server) | `@tanstack/react-query` |
| State (client) | `zustand` (ถ้าจำเป็น), prefer URL params + Server Components |
| Forms | `react-hook-form` + `zod` |
| Charts | `recharts` |
| Tables | `@tanstack/react-table` |
| Date | `date-fns` (ไม่ใช้ moment.js) |
| i18n | `next-intl` |
| Toasts | `sonner` |
| Icons | `lucide-react` |
| Package mgr | `pnpm` |
| Test | `vitest` + `@testing-library/react`, `playwright` สำหรับ e2e |

### Conventions
- Server Components by default
- `'use client'` เฉพาะที่ต้อง interactive
- Mutations ผ่าน Server Actions หรือ React Query mutation
- ห้าม fetch ใน `useEffect` — ใช้ React Query หรือ RSC

---

## Database — PostgreSQL 16

### เหตุผล
- **JSONB** สำหรับ flexible data (e.g. signature details, scan metadata)
- **Full-text search** built-in — software search ไม่ต้องพึ่ง Elasticsearch
- **Partitioning** สำหรับ history tables (scans, alerts)
- **มี indices type หลายอย่าง** — GIN, BTREE, BRIN

### Conventions
- Schema name: `public`
- Table naming: snake_case, plural (`machines`, `software_records`)
- Primary key: `id BIGSERIAL` หรือ `id UUID DEFAULT gen_random_uuid()` (เลือก UUID สำหรับ entity ที่ expose ใน API)
- Timestamps: `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`, `updated_at TIMESTAMPTZ` (auto-update ด้วย trigger)
- Soft delete: ใช้ `deleted_at TIMESTAMPTZ NULL` (ห้ามลบ row จริงสำหรับ entity ที่มี audit log)

---

## Infrastructure

| ใช้ทำ | Choice |
|--------|--------|
| Container | Docker + Compose |
| Reverse proxy | Nginx |
| TLS | Let's Encrypt via Certbot (prod) / self-signed (dev) |
| Background jobs | `arq` worker (Redis-backed) |
| Cache | Redis 7 |
| Monitoring | Prometheus + Grafana |
| Log aggregation | (out of scope v1) — write to stdout, capture ด้วย Docker logs |

---

## ไม่ใช้

| Reject | เหตุผล |
|--------|--------|
| ❌ Kubernetes | Overkill สำหรับ single tenant. Docker Compose พอ |
| ❌ Microservices | 1 backend monolith ง่ายกว่าและพอ |
| ❌ GraphQL | REST + OpenAPI ชัดและพอ |
| ❌ tRPC | Backend เป็น Python, ไม่ share type ได้อยู่แล้ว |
| ❌ Prisma / ORM อื่น | SQLAlchemy 2 mature สุดใน Python |
| ❌ Redux | Server state ใช้ React Query, client state ส่วนใหญ่ไม่ต้อง store |
| ❌ Elasticsearch | Postgres FTS พอ |
| ❌ Kafka | ใช้ Redis pubsub ผ่าน `arq` พอ |
