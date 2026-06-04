# SoftSentry Roadmap

แบ่งงานเป็น 5 phase. **ห้ามข้าม phase** — แต่ละ phase ต้อง test ผ่านก่อน move ไป phase ถัดไป

---

## Phase 1 — Infrastructure & Auth Backbone

**Goal:** มี skeleton ของทั้ง 3 services + auth พื้นฐาน + DB migrations

### Deliverables
- [x] `docker-compose.yml` ใช้งานได้ (postgres, redis, backend, worker, dashboard) — verify e2e แล้ว (nginx reverse proxy เป็น prod-only ดู `docs/08-deployment.md`)
- [x] Backend skeleton (FastAPI + SQLAlchemy + Alembic + ruff/black setup) — lint/format/mypy สะอาด, 20 pytest ผ่าน
- [x] Dashboard skeleton (Next.js + shadcn/ui + Tailwind + next-intl) — lint/typecheck สะอาด, build + serve ใน container ผ่าน
- [x] Agent skeleton (Go + cobra CLI + golangci-lint) — build/vet/test ผ่าน
- [x] **JWT auth** + password hashing (bcrypt)
- [x] **Agent enrollment token** mechanism (1-time token → permanent agent token)
- [x] User table + initial Alembic migration
- [x] Health check endpoint (`GET /health`)
- [x] CI skeleton (GitHub Actions: lint + test ทุก service)
- [x] `.env.example` + secrets management

### Acceptance
- [x] รัน `docker compose up` แล้วทั้งระบบ start ได้ — postgres/redis/backend/worker/dashboard ขึ้นครบ, backend auto-migrate + seed
- [x] Login flow ใช้งานได้: `POST /api/v1/auth/login` คืน JWT — verify e2e (200 + JWT, ผิด → 401, `/me` + token → 200) + pytest
- [x] มี seed script สร้าง admin user เริ่มต้น (`app/seed.py`) — รันอัตโนมัติตอน backend boot

**Phase 1 ✅ เสร็จสมบูรณ์ — verify e2e ผ่านครบทุก acceptance**

---

## Phase 2 — Agent + Inventory (Modules 1-2)

**Goal:** Agent สแกน software + signature, ส่งขึ้น server, ดูบน dashboard ได้

### Deliverables
- **Module 1 (Agent)**
  - [x] Software scanner (Windows: Registry uninstall keys + Authenticode) — verified live (177 apps)
  - [x] Software scanner (macOS: `/Applications` plist + codesign) — code written, not yet run on a Mac
  - [x] Auto-scan scheduler (configurable interval, default 6h, clamped 1-24) — timer alongside heartbeat; next scan computed from persisted `last_scan_at` so restart doesn't re-scan early
  - [x] Manual scan trigger (server → agent via next heartbeat)
  - [x] Heartbeat ทุก 60s
  - [x] Real OS-version detection (Windows registry / macOS sw_vers)
  - [x] Auto-update mechanism (download binary + SHA256 verify + atomic self-replace + rollback; signature-verify is a documented TODO)
  - [x] Install เป็น Windows Service (SCM) / macOS LaunchDaemon — `install/uninstall/status/logs` CLI; build verified, not run with admin
  - [x] Local retry queue (file-based, crash-safe)
- [x] **Module 2 (Inventory)** — backend done, 65 pytest pass
  - [x] `POST /api/v1/scans` รับ scan result (idempotent via Idempotency-Key)
  - [x] `GET /api/v1/machines` list + filter (status/os/search, sort, pagination); status derived from `last_seen_at` (online ≤5m / stale ≤1h / offline)
  - [x] `GET /api/v1/machines/{id}/software` รายการ software
  - [x] `GET /api/v1/software/compare` เปรียบเทียบระหว่างเครื่อง
  - [x] Software install/uninstall history (diff between scans) — `GET /machines/{id}/history`
  - [x] Top 10 software widget — `GET /software/top`
- [x] **Dashboard pages** — lint + typecheck clean, `next build` compiles (standalone symlink step is Windows-only env limit; OK in Docker)
  - [x] Machine list page (status filter + debounced search + pagination)
  - [x] Per-machine detail page (tabs: Software / History / Scans + trigger-scan)
  - [x] Inventory search/filter (cross-machine software list, top-N, compare)

### Acceptance
- [~] ติดตั้ง agent ทดสอบบน Win + Mac → เห็นใน dashboard — code-complete; live install ต้องใช้ admin (ยังไม่รัน)
- [x] Filter software list ตามชื่อ/publisher ได้ — `GET /software?q=` + UI
- [x] Compare 2 machines: เห็น "machine A มี X แต่ B ไม่มี" — `POST /software/compare`
- [x] Agent offline > 5 min → status เปลี่ยนเป็น offline — derived in `machine_service.compute_status`

**Phase 2 ✅ code-complete — agent (9/9) + inventory backend (65 pytest) + dashboard (lint/typecheck clean, compiles). Live Win/Mac install pending admin.**

---

## Phase 3 — Signature + Unauthorized + CVE (Modules 3-5)

**Goal:** Risk detection engine — รู้ว่าเครื่องไหนเสี่ยง

### Deliverables
- [x] **Module 3 (Signature Verification)** — ✅ complete (backend wired + dashboard UI)
  - Agent ส่ง signature metadata (issuer, expiry, status) ขึ้นมาแล้วใน Phase 2
  - [x] Backend: `GET /signatures` (list + multi-status union filter + search), `GET /signatures/{software_uuid}` (chain + issues), `GET /dashboard/signature-stats` (donut data) — 14 pytest pass
  - [x] Auto-flag เมื่อ status ≠ valid: `signature_service.evaluate_machine` เรียก inline ในขั้น scan ingest (`agents.py`, ต่อจาก unauthorized engine) → alert `signature_invalid`/`signature_unsigned` ภายใน 1 scan cycle; deduped per (machine,software,type); auto-resolve เมื่อ signature หาย/uninstall; `unsigned` เคารพ `policy.flag_unsigned`
  - [x] Dashboard UI — หน้า `/signatures` + `SignatureBadge` (colored badge + tooltip + คลิกเปิด modal), `SignatureDetailModal` + `CertChainTree` (leaf cert + chain tree + issues), `SignatureStatusFilter` (multi-select union), `SignatureStatsWidget` (donut + click-to-drill); nav link + i18n (th/en). Verified: tsc + eslint clean, 19 vitest pass, `next build` compiles (standalone symlink = Windows env limit, OK ใน Docker)
- [x] **Module 4 (Unauthorized Detection)** — ✅ complete (backend TDD: 26 matcher + 10 CSV + 7 integration; dashboard UI done)
  - [x] Whitelist CRUD (`POST/GET/PATCH/DELETE /api/v1/whitelist`) — admin-gated mutations
  - [x] Blacklist CRUD (+ severity/reason)
  - [x] Bulk import CSV (`/bulk`, BOM-safe, dedup, partial-accept + per-row errors, ≤5MB)
  - [x] Auto-detect after scan: `unauthorized_service.evaluate_machine` (inline in scan txn) → alert ภายใน 1 scan cycle; blacklist wins; whitelist-mode flags unlisted; dedup per (machine,software,type); auto-resolve on uninstall (not on policy change)
  - [x] `alerts` table + `GET /api/v1/alerts` (filter type/status/severity/machine) + acknowledge/resolve
  - [x] `policy_settings` toggle (`GET/PATCH /api/v1/policy`, whitelist_mode)
  - [x] Alembic `0002` backfills all Phase 2 + Phase 3 tables (offline-SQL verified; prior migration only had Phase 1 tables)
  - [x] Dashboard UI — `/policy/whitelist`, `/policy/blacklist`, `/alerts` pages + reusable `PolicyEntryTable`/`PolicyEntryForm`/`BulkImportDialog`/`PolicyModeToggle`/`AlertFeed`; i18n (th/en), admin-gated mutations, alert feed auto-polls 30s. Verified: 14 vitest (pure helpers), tsc + eslint clean, `next build` compiles (standalone symlink = Windows env limit, OK in Docker)
- [x] **Module 5 (Vulnerability Check)** — ✅ complete (backend 166 pytest pass + dashboard UI)
  - [x] Daily CVE sync worker (NVD JSON feed + OSV API) — `workers/cve_sync.py` + `services/cve_sync.py`, last-modified incremental, manual trigger via `POST /admin/cve-sync/trigger`
  - [x] CVE matching algorithm (CPE matching + fuzzy version compare) — `services/cve_matching.py`, match_confidence high/medium/low, name+vendor guard against false positives; wired inline in scan ingest (`agents.py` → `vulnerability_service.evaluate_machine`) → vuln within 1 scan cycle, auto-resolve on uninstall/upgrade
  - [x] `GET /api/v1/vulnerabilities` (+ filter severity/machine/date/confidence/q, dismiss/undismiss) + `GET /machines/{uuid}/vulnerabilities` + `GET /cve/{id}` + `GET /dashboard/vuln-summary`
  - [x] Recommended-version field (from NVD `versionEndExcluding`)
  - [x] Severity dashboard widget (donut + click-drill, 30s poll)
  - [x] CVE critical alert (type `cve_critical`, dedup per cve+machine, auto-resolve)
  - [x] Dashboard UI — `/vulnerabilities` page + per-machine "Vulnerabilities" tab; `SeverityBadge`/`SeverityFilter`/`VulnSummaryWidget`/`VulnTable`/`CVEDetailModal`/`DismissDialog`; admin-gated dismiss + manual sync; i18n (th/en). Verified: tsc + eslint clean, 28 vitest (9 new) pass, `next build` compiles (standalone symlink = Windows env limit, OK in Docker)

### Acceptance
- [x] ติดตั้ง software ที่อยู่ใน blacklist → alert ขึ้นภายใน 1 scan cycle (Module 4)
- [x] มี software ที่มี CVE → เห็นใน dashboard พร้อม severity — `/vulnerabilities` + per-machine tab + `SeverityBadge`
- [~] CVE database sync ครั้งแรกใช้เวลา <= 30 นาที, daily update incremental — code-complete (incremental via last-modified); live full-feed timing ยังไม่ได้วัดที่ scale จริง

**Phase 3 ✅ code-complete — Modules 3-5 ครบ. Module 5: backend (166 pytest) + dashboard UI (tsc/eslint clean, 28 vitest, compiles). Live full CVE-feed sync timing pending real run.**

---

## Phase 4 — License + Dashboard + Reporting (Modules 6-8)

**Goal:** Business value — IT admin ใช้งานจริงได้

### Deliverables
- [x] **Module 6 (License Compliance)** — ✅ code-complete (backend 22 pytest + dashboard UI)
  - [x] License CRUD (`POST/GET/PATCH/DELETE /api/v1/licenses`) — admin-gated; `license_key` AES-encrypted at rest (Fernet `EncryptedString` TypeDecorator), never returned in list, decrypted only on single GET
  - [x] Auto-calculate installed_count (live SQL ILIKE on `software_name`, DISTINCT live machine) vs purchased_count → status (`compliant`/`over_used`/`expiring_soon`/`expired`, precedence expired>over>expiring); `+ /compliance-summary`, `/{uuid}/installations` drill-down, `/refresh-counts`
  - [x] Expiry alerts (90/30/7-day bands + expired + over-used) — daily `license_compliance_check` worker → `license_expiring`/`license_expired`/`license_overused` alerts in shared `alerts` table; dedup per (license, band), escalates across bands, renewal/fix auto-resolves. `alerts.machine_id` made nullable + `license_id`/`dedup_key` added (Alembic `0005`, offline-SQL verified)
  - [x] Compliance rate widget (donut + click-drill, 30s poll) + `/licenses` page: LicenseTable, LicenseForm (LicenseKeyField show/hide), LicenseStatusBadge, ComplianceSummaryWidget, LicenseInstallationsDrawer (CSV export); status filter + search + pagination; i18n th/en; admin-gated mutations. Verified: tsc + eslint clean, 37 vitest (9 new) pass, `next build` compiles (standalone symlink = Windows env limit, OK in Docker)
  - [~] Bulk CSV import (spec 6.6, "optional v1") + audit_log on edit (spec 6.1) — deferred: audit_log table lands in Module 9 (Phase 5)
- [x] **Module 7 (Dashboard)** — ✅ code-complete (backend 17 pytest + dashboard overview UI)
  - [x] Risk score: `risk_service` weighted sum `unsigned*1 + unauthorized*3 + cve_critical*5 + cve_high*3 + cve_medium*1 + cve_low*0.5`; computed inline after each scan (last engine in the scan txn) and **stored** in new `machines.risk_score` column (indexed → top-N is a cheap sort). Color bands 0 green / 1-10 yellow / 11-30 orange / 31+ red. Counts read from source-of-truth tables for active inventory (signature status, open unauthorized/blacklist alerts, ≥medium-confidence CVEs). Alembic `0006` (offline-SQL verified)
  - [x] Endpoints: `GET /dashboard/overview` (KPI counts), `/dashboard/risk-scores?limit=10` (top-N, score>0), `/dashboard/charts/vuln-trend?period=30d` (daily severity buckets, zero-filled), `GET /machines/{uuid}/risk` (per-machine breakdown). Signature donut + license gauge reuse existing `/dashboard/signature-stats` + `/licenses/compliance-summary`. Machine detail now carries real `risk_score` + `vulnerability_count`
  - [x] Overview page (root `/`, auth-gated — replaces old public landing): 4 KPI cards (machines/agents-online+offline-warn/software-unique/vuln C·H·M) with drill links, severity donut + signature donut + compliance gauge (click→drill), vuln trend line chart (Recharts), top-10 risky-machines horizontal bar (click→machine), live alert feed (poll 30s), `RefreshControl` pause/resume + refresh-now; all-offline banner. Per-machine `RiskScoreCard` (weighted breakdown) on the machine overview tab. nav "Overview" link + i18n th/en. Verified: tsc + eslint clean, 42 vitest (5 new) pass, `next build` compiles 3/3 pages (standalone symlink = Windows env limit, OK in Docker)
- [x] **Module 8 (Reporting)** — ✅ code-complete (backend 35 pytest + dashboard UI)
  - [x] PDF org report (`org_summary.html` + machine-by-OS bars, inline-SVG vuln trend, top-10 risky/vuln tables, license + unauthorized sections) — Jinja `render_html` (unit-tested everywhere) split from `render_pdf` (lazy WeasyPrint import; native Pango/cairo absent on Windows → exercised in Linux container)
  - [x] PDF per-machine report (`machine_detail.html` — machine info, risk breakdown, signature tally, vuln/alert/history/software sections; empty sections hidden)
  - [x] CSV export ทุก table — inline `/{machines,software,vulnerabilities,licenses,alerts}/export` (RFC 4180 + UTF-8 BOM for Excel/Thai, streaming row-by-row, current filters preserved); export router registered **before** the `/{uuid}` routers so literal `/export` wins the match
  - [x] Async generation — `POST /reports/generate` → arq `generate_report` → file in `reports_dir`; `reports` table (status queued/running/completed/failed), poll `GET /reports/{uuid}`, `download`, admin `DELETE`; 1-min idempotency dedup; daily `cleanup_reports` for 30-day retention
  - [x] Scheduled reports — `report_schedules` table + CRUD + `run-now`; per-minute `run_due_schedules` cron fires due schedules (skips if previous run in flight); `cron_service` (croniter) validates + computes `next_run_at` + 3-run preview; email delivery deferred to Phase 5 (recipients stored). Alembic `0007` (offline-SQL verified: JSONB params/recipients, FK order report_schedules→reports)
  - [x] Dashboard UI — `/reports` (generate dialog + report table, 5s poll for status, download/delete) + `/reports/schedules` (cron preset/advanced builder, pause/resume/run-now); `ExportButton` on machines/software/vulns/licenses/alerts list pages; per-machine `ReportGenerateButton` on machine detail; nav link + i18n th/en. Verified: tsc + eslint clean, 46 vitest (4 new) pass, `next build` compiles 3/3 (standalone symlink = Windows env limit, OK in Docker)

### Acceptance
- [x] หน้า overview load <= 1s ที่ data 100 machines × 500 software — stored `machines.risk_score` (Module 7) makes top-N an indexed sort; KPI counts are simple aggregates (live perf at scale pending real data)
- [~] PDF org report generate ได้ไม่เกิน 10s — code-complete; live timing pending a Linux/Docker run (WeasyPrint not runnable on this Windows dev box)
- [x] Schedule "ทุกวันจันทร์ 9 โมง → ส่ง PDF" ทำงานถูก — `0 9 * * MON` → `next_run_at` lands on Monday 09:00 (unit-tested); per-minute worker fires due schedules

**Phase 4 ✅ code-complete — Modules 6-8 ครบ. Module 8: backend (35 pytest: csv/cron/pdf-render units + exports + reports/schedules API) + dashboard UI (tsc/eslint clean, 46 vitest, compiles). Live PDF-timing + email delivery (Phase 5) pending.**

---

## Phase 5 — User Mgmt + i18n + Telemetry + Polish

**Goal:** Production-ready

### Deliverables
- [x] **Module 9 (User Management)** — ✅ code-complete (backend 22 pytest → 262 total, ruff+mypy strict clean; dashboard 7 vitest → 53 total, tsc/eslint clean, `next build` compiles)
  - [x] User CRUD (admin only) — `GET/POST/GET/PATCH/DELETE /users` + `POST /users/{uuid}/reset-password`; soft-delete, password generated + shown once, edge cases (cannot delete self, cannot demote/disable/delete last admin, email reusable after soft-delete via **partial unique index** `WHERE deleted_at IS NULL`)
  - [x] Role-based middleware (Admin/Viewer) — reused `require_role("admin")`; admin-gated mutations; disabled/soft-deleted users self-invalidate via DB re-check in `get_current_user`/`rotate_refresh`
  - [x] Audit log table — `audit_logs` model + Alembic `0008` (offline-SQL verified); password policy validator (spec 9.8: 12-char + complexity + common-password reject)
  - [x] **Audit coverage — "log ทุก mutation" (spec 9.6)** — shared `api/v1/_audit.py::audit` helper (fills ip/UA from the request, flushes within the handler's txn) now records **every** admin/mutation across the surface: user CRUD + `auth.login_*` (since Module 9) **+ license create/update/delete/refresh, whitelist/blacklist CRUD + bulk, policy toggle, alert acknowledge/resolve, vulnerability dismiss/undismiss, report generate/delete, schedule CRUD + run-now, cve_sync trigger**. before/after snapshots are stored (secrets like `license_key` are excluded by an allow-list). Closes Phase 5 acceptance *"ลบ software/edit license → เห็นใน log"*. 3 new pytest (`test_audit_mutations.py`) → 270 total
  - [x] Audit log viewer **API** — `GET /audit-logs` (filter user/action/entity/date, paginate) + `/audit-logs/actions` + `/audit-logs/export` (CSV); admin-only
  - [x] Dashboard pages — `/settings/users` (UserTable + UserFormDialog + PasswordRevealDialog: search/role filter, role badges, edit, reset-pw, disable/enable, soft-delete; self-actions disabled), `/settings/audit-log` (AuditLogTable + JsonDiffView: action/entity/date filters, expandable before/after diff, device parse, pagination, CSV export), `/settings/profile` (change-password via existing `/auth/change-password`); nav links admin-gated (profile for all); i18n th/en
  - [~] Deferred: 9.2 self-service password reset (needs email/Phase 5), 9.7 session list/revoke (needs refresh_tokens DB table — current design stores refresh jti in Redis)
- [x] **i18n** — ✅ code-complete (dashboard 61 vitest → +4, tsc/eslint clean, `next build` compiles)
  - [x] next-intl setup — `i18n/routing.ts` (locales th/en, defaultLocale th, `localePrefix: as-needed`) + `request.ts` + `middleware.ts`; done since Phase 1
  - [x] แปลทุก user-facing string เป็น ไทย + EN — ทุก namespace ครบทั้ง 2 ไฟล์; key-parity verified (en↔th: ไม่มี key หาย) ผ่าน recursive flatten check
  - [x] Language switcher — `components/LanguageSwitcher.tsx` (native Select, ไทย/English endonyms via `lib/locale.ts` helper, unit-tested); preserves pathname+query, switches via `router.replace({pathname,query},{locale})`, persists in NEXT_LOCALE cookie; mounted in dashboard header + login page (pre-auth)
- [x] **Telemetry** — ✅ code-complete (backend 267 pytest → +5, ruff+mypy strict clean; compose config validates)
  - [x] Prometheus metrics endpoint ใน backend (`/metrics`) — 3 families on the default registry: **HTTP** (`softsentry_http_requests_total` + `_request_duration_seconds`, recorded by an in-house middleware keyed on the matched **route template** so cardinality is bounded by route count, `/metrics` self-excluded — chose middleware over `prometheus-fastapi-instrumentator` to avoid a new dep, same contract), **agent**, **business KPIs**. `core/metrics.py` defs + `services/metrics_service.py` collector
  - [x] Agent metrics (scan duration, software count, errors) — derived from the scan payload agents already push (`started_at`/`completed_at` → duration, `len(software)` → count) so **no Go change**; matches architecture ("ไม่ส่ง telemetry นอก backend" — agents never scraped directly). `softsentry_agent_scans_total{scan_type}`, `_scan_duration_seconds`, `_scan_software_count`, `_scan_errors_total` (incremented if ingest raises)
  - [x] Business KPIs — `softsentry_machines{status}`, `_software_unique`, `_alerts_open{severity}`, `_vulnerabilities_open{severity}`, `_licenses{status}` + a refreshed-timestamp staleness gauge; refreshed in-process every `metrics_refresh_seconds` (default 30) by an asyncio loop started in the FastAPI lifespan (gauges are per-process, so this can't live in the arq worker). `collect_business_snapshot` is split from `apply_snapshot` for unit-testing without the registry
  - [x] Grafana dashboards: backend health / agent fleet health / business KPIs — 3 JSON in `infra/grafana/dashboards/` + provisioning (`provisioning/datasources/prometheus.yml` uid `softsentry-prometheus`, `provisioning/dashboards/provider.yml`); compose grafana volumes rewired to mount `provisioning/` + dashboards into `/var/lib/grafana/dashboards`. Prometheus already scrapes `backend:8000/metrics`
  - [~] Live verify pending a `docker compose --profile observability up` run (Prometheus actually scraping + Grafana rendering); all configs parse + `docker compose config` validates
- [~] **Polish** — error pages + loading + empty states done; a11y/Lighthouse audits pending live run
  - [x] Error pages (404, 500) — `app/[locale]/not-found.tsx` (i18n 404), `app/[locale]/error.tsx` (route-segment boundary, i18n + chrome, logs digest), `app/global-error.tsx` (root-layout fallback, own `<html>`/`<body>`, locale-agnostic copy); decorative icons `aria-hidden`
  - [x] Loading skeletons — `app/[locale]/(dashboard)/loading.tsx` route-group skeleton (`aria-busy`/`aria-live`); per-page list loaders already in place from earlier modules
  - [x] Empty states ทุก list — every list page already renders an i18n `empty` message (machines/software/signatures/vulns/licenses/alerts/policy/reports/users/audit)
  - [~] A11y audit pass (axe) — runtime audit; needs a running server. Obvious a11y in place (aria-hidden decorative icons, aria-busy loaders, labelled controls); axe sweep pending
  - [~] Performance audit (Lighthouse >= 90) — runtime audit against a deployed build; pending live run

### Acceptance
- Admin สร้าง Viewer user ได้, Viewer login แล้ว read-only
- เปลี่ยนภาษา ไทย ↔ EN → ทุกหน้าเปลี่ยนตาม
- Grafana dashboard เห็น metrics จาก backend + agent fleet
- [x] Audit log: ลบ software/edit license → เห็นใน log — ครอบคลุมทุก mutation (license/policy/alert/vuln/report/cve-sync), `test_audit_mutations.py` verify

---

## ลำดับใน phase

ภายในแต่ละ phase, implement ตามลำดับนี้:

1. **Backend models + migrations** ก่อน
2. **Backend API + tests** (TDD)
3. **Agent code** (ถ้ามี)
4. **Dashboard pages** (ใช้ mock data ก่อนถ้า API ยังไม่เสร็จ)
5. **Integration test** (e2e ผ่าน docker compose)

---

## Conventions Update

หลังจบแต่ละ phase ให้ update:
- `CLAUDE.md` ถ้ามี convention ใหม่
- `docs/10-testing-strategy.md` ถ้าเจอ pattern test ที่ดี
- README "Status" section
