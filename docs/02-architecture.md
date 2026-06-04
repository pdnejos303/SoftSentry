# 02 — System Architecture

## High-level diagram

```
┌──────────────────────────────────────────────────────────────────┐
│                         Endpoint Fleet                            │
│                                                                   │
│   ┌──────────────┐    ┌──────────────┐    ┌──────────────┐        │
│   │ Windows PC   │    │ Windows PC   │    │   macOS      │  ...   │
│   │  ┌────────┐  │    │  ┌────────┐  │    │  ┌────────┐  │        │
│   │  │ Agent  │  │    │  │ Agent  │  │    │  │ Agent  │  │        │
│   │  └───┬────┘  │    │  └───┬────┘  │    │  └───┬────┘  │        │
│   └──────┼───────┘    └──────┼───────┘    └──────┼───────┘        │
│          │                   │                   │                 │
└──────────┼───────────────────┼───────────────────┼─────────────────┘
           │                   │                   │
           │   HTTPS + Bearer token (agent token)  │
           │                   │                   │
           ▼                   ▼                   ▼
    ┌─────────────────────────────────────────────────────┐
    │                    Nginx (TLS)                       │
    │              /api/* → backend,  /* → dashboard       │
    └─────────────────────────────┬───────────────────────┘
                                  │
            ┌─────────────────────┼─────────────────────┐
            ▼                     ▼                     ▼
    ┌──────────────┐    ┌──────────────────┐   ┌──────────────────┐
    │  Dashboard   │    │  Backend API     │   │   Worker(s)      │
    │  (Next.js)   │    │  (FastAPI)       │   │   (arq + Redis)  │
    │              │    │                  │   │                  │
    │  - SSR       │    │  /api/v1/*       │   │  - CVE sync      │
    │  - i18n      │    │  - auth (JWT)    │   │  - Alert scan    │
    │  - Charts    │    │  - inventory     │   │  - License check │
    └──────────────┘    │  - vuln          │   │  - Report gen    │
                        │  - license       │   └────────┬─────────┘
                        │  - report        │            │
                        └────────┬─────────┘            │
                                 │                      │
                  ┌──────────────┴──────────────────────┴───┐
                  ▼                                          ▼
            ┌──────────────┐                          ┌──────────────┐
            │  PostgreSQL  │                          │   Redis      │
            │              │                          │  (cache +    │
            │  - users     │                          │   job queue) │
            │  - machines  │                          └──────────────┘
            │  - software  │
            │  - cve       │                          ┌──────────────┐
            │  - licenses  │◀─────CVE sync────────────│  NVD / OSV   │
            │  - alerts    │                          │  (external)  │
            │  - audit_log │                          └──────────────┘
            └──────────────┘
                  │
                  ▼
            ┌──────────────────┐
            │  Prometheus +    │
            │  Grafana         │
            │  (metrics)       │
            └──────────────────┘
```

---

## Component Responsibilities

### Agent (Go)
- Discover installed software (per-OS)
- Verify digital signatures
- Send periodic scan results (default ทุก 6 ชม.)
- Heartbeat (60s)
- Pull config updates from backend (scan interval, etc.)
- Self-update binary

**ห้ามทำ:**
- ไม่อ่าน user data (browser history, documents)
- ไม่ส่ง telemetry นอก backend
- ไม่ execute commands จาก server (ไม่เป็น RAT)

### Backend API (FastAPI)
- Authentication (JWT) for dashboard users
- Agent enrollment + token validation
- CRUD APIs for ทุก entity
- Business logic: risk scoring, license matching, CVE matching
- Sync external CVE feeds
- Generate reports

**ห้ามทำ:**
- ไม่ execute long-running tasks ใน request — โยนเข้า worker queue
- ไม่ store secrets ใน DB (ใช้ env var + secrets manager)

### Worker (arq)
- CVE feed sync (daily, NVD + OSV)
- Match new scan results vs whitelist/blacklist → emit alerts
- License expiry check (daily)
- Scheduled report generation
- Email/notification dispatch (Phase 5)

### Dashboard (Next.js)
- All UI — read-only ใน Viewer role, full CRUD ใน Admin role
- SSR สำหรับ initial page load
- Client-side navigation + React Query สำหรับ refetch
- i18n (ไทย/EN)

### PostgreSQL
- Single source of truth สำหรับทุกข้อมูล

### Redis
- Job queue (arq backend)
- Cache (CVE lookup, machine list — TTL สั้น)
- Rate limiting (agent enrollment, login attempts)

---

## Data Flow Scenarios

### Scenario 1: Agent scan
```
Agent → POST /api/v1/scans (เนื้อหา: machine_id + scan results)
Backend → validate agent token
       → store ใน DB (software_records, signature_records)
       → enqueue job: "match_alerts" (whitelist/blacklist) + "match_cve"
       → return 200
Worker → ดึง job
       → query DB → match → insert alerts
```

### Scenario 2: Dashboard query
```
User → browser → Next.js SSR
Next.js → fetch /api/v1/machines (Server Component)
Backend → query DB → return JSON
Next.js → render HTML → send to browser
Browser → React hydrate → React Query takes over for subsequent fetch
```

### Scenario 3: CVE sync
```
Cron schedule (arq) → trigger job ทุก 24 ชม.
Worker → fetch NVD JSON feed (incremental)
       → parse + insert/update `cve` table
       → enqueue job "rematch_all_machines"
Worker (rematch) → loop ทุก machine → match → update vulnerabilities table
```

---

## Trust Boundaries

| Boundary | Protocol | Auth |
|----------|----------|------|
| Agent → Backend | HTTPS | Agent token (Bearer) |
| Dashboard → Backend | HTTPS | JWT (Bearer, in `Authorization` header) |
| Backend → Postgres | Internal Docker network | Username/password (env) |
| Backend → Redis | Internal Docker network | Password (env) |
| Backend → NVD/OSV | HTTPS outbound | None (public API) |
| Prometheus → Backend | Internal Docker network | None (private network) |

**Trust assumption:** Docker internal network ถือเป็น trusted. ถ้า attacker ได้ inside network = ถือว่า game over

---

## Scaling Considerations (v1: single instance)

- 1 backend instance รับได้ ~1,000 agents (heartbeat 60s = ~17 req/s)
- Postgres handle 1,000 agents × scan ทุก 6 ชม. = ~3 scan/min — sustainable แน่นอน
- Bottleneck แรกที่จะเจอ: dashboard query บน 100,000+ software records → ต้อง index ให้ครบ

**Scale out plan (v2):**
- Add backend replica behind Nginx round-robin
- Postgres read replica สำหรับ dashboard
- Move worker ไป dedicated instance

ไม่ optimize ก่อนเวลา — measure แล้วค่อยทำ
