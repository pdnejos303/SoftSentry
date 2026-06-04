# 03 — Data Model

> Schema สำหรับ PostgreSQL 16. Alembic migrations จะถูก generate จาก SQLAlchemy models ใน `backend/app/models/`

## Conventions
- Table names: `snake_case`, plural (`machines`, `software_records`)
- PK: `id BIGSERIAL` ภายใน, `uuid UUID UNIQUE` ที่ expose ใน API
- Timestamps: `created_at`, `updated_at` (TIMESTAMPTZ NOT NULL DEFAULT NOW())
- Soft delete: `deleted_at TIMESTAMPTZ NULL` สำหรับ entity ที่มี audit
- FK: `<table>_id` (singular), `ON DELETE` ระบุชัดเสมอ
- Booleans: `is_*` หรือ `has_*`
- JSON columns: `JSONB` ไม่ใช้ `JSON`

---

## Entity Relationship Overview

```
users ──┬──▶ audit_logs
        └──▶ alert_acks

machines ──┬──▶ scans ──┬──▶ software_records
           │            └──▶ signature_records
           ├──▶ heartbeats
           └──▶ agent_configs

software_records ──┬──▶ alerts
                   └──▶ vulnerabilities (via CVE match)

cve_records ──▶ vulnerabilities ──▶ software_records

whitelist_entries
blacklist_entries
                ──▶ alerts

licenses ──▶ license_installations ──▶ software_records
```

---

## Tables

### `users`
ผู้ใช้งาน dashboard

| Column | Type | Notes |
|--------|------|-------|
| id | BIGSERIAL PK | |
| uuid | UUID UNIQUE NOT NULL | API-exposed identifier |
| email | VARCHAR(255) UNIQUE NOT NULL | login id |
| password_hash | VARCHAR(255) NOT NULL | bcrypt |
| full_name | VARCHAR(255) NOT NULL | |
| role | VARCHAR(20) NOT NULL | `admin` หรือ `viewer` (CHECK constraint) |
| is_active | BOOLEAN NOT NULL DEFAULT TRUE | |
| last_login_at | TIMESTAMPTZ NULL | |
| created_at | TIMESTAMPTZ NOT NULL DEFAULT NOW() | |
| updated_at | TIMESTAMPTZ NOT NULL DEFAULT NOW() | |
| deleted_at | TIMESTAMPTZ NULL | soft delete |

**Indexes:** `email` (unique), `uuid` (unique)

---

### `machines`
เครื่อง endpoint ที่ลง agent

| Column | Type | Notes |
|--------|------|-------|
| id | BIGSERIAL PK | |
| uuid | UUID UNIQUE NOT NULL | |
| hostname | VARCHAR(255) NOT NULL | |
| os | VARCHAR(20) NOT NULL | `windows` / `macos` |
| os_version | VARCHAR(50) NOT NULL | |
| arch | VARCHAR(10) NOT NULL | `amd64` / `arm64` |
| agent_version | VARCHAR(20) NOT NULL | |
| agent_token_hash | VARCHAR(255) UNIQUE NOT NULL | bcrypt hash ของ agent token |
| enrolled_at | TIMESTAMPTZ NOT NULL DEFAULT NOW() | |
| last_seen_at | TIMESTAMPTZ NULL | updated ทุก heartbeat |
| last_scan_at | TIMESTAMPTZ NULL | |
| status | VARCHAR(20) NOT NULL DEFAULT 'online' | `online`/`offline`/`stale` |
| tags | JSONB NOT NULL DEFAULT '[]'::jsonb | array ของ string สำหรับ grouping |
| created_at | TIMESTAMPTZ | |
| updated_at | TIMESTAMPTZ | |
| deleted_at | TIMESTAMPTZ NULL | |

**Indexes:** `uuid`, `hostname`, `last_seen_at` (สำหรับ status query), `(status, last_seen_at)`

---

### `agent_configs`
Config ที่ agent pull จาก server

| Column | Type | Notes |
|--------|------|-------|
| machine_id | BIGINT PK FK → machines.id ON DELETE CASCADE | 1:1 |
| scan_interval_hours | INTEGER NOT NULL DEFAULT 6 | 1-24 |
| auto_update_enabled | BOOLEAN NOT NULL DEFAULT TRUE | |
| manual_scan_requested | BOOLEAN NOT NULL DEFAULT FALSE | server toggle เพื่อสั่ง scan manual |
| updated_at | TIMESTAMPTZ | |

---

### `heartbeats`
> เก็บแค่ 7 วันล่าสุด (partitioned by week, drop partition เก่า)

| Column | Type | Notes |
|--------|------|-------|
| id | BIGSERIAL PK | |
| machine_id | BIGINT FK → machines.id ON DELETE CASCADE | |
| received_at | TIMESTAMPTZ NOT NULL DEFAULT NOW() | |
| agent_version | VARCHAR(20) | |

**Indexes:** `(machine_id, received_at DESC)`

---

### `scans`
1 row per ครั้งที่ agent ส่ง scan ขึ้นมา

| Column | Type | Notes |
|--------|------|-------|
| id | BIGSERIAL PK | |
| uuid | UUID UNIQUE NOT NULL | |
| machine_id | BIGINT FK → machines.id ON DELETE CASCADE | |
| started_at | TIMESTAMPTZ NOT NULL | agent-side timestamp |
| completed_at | TIMESTAMPTZ NOT NULL | |
| received_at | TIMESTAMPTZ NOT NULL DEFAULT NOW() | server-side |
| software_count | INTEGER NOT NULL | |
| scan_type | VARCHAR(20) NOT NULL | `auto` / `manual` |
| trigger | VARCHAR(20) | `schedule` / `user_request` / `enroll` |

**Indexes:** `(machine_id, received_at DESC)`

---

### `software_records`
Software ที่เจอใน scan ล่าสุดของแต่ละเครื่อง.
ใช้ **upsert** จาก scan: ถ้ามีแล้ว update `last_seen_scan_id`, ถ้าไม่มี insert

| Column | Type | Notes |
|--------|------|-------|
| id | BIGSERIAL PK | |
| uuid | UUID UNIQUE NOT NULL | |
| machine_id | BIGINT FK → machines.id ON DELETE CASCADE | |
| name | VARCHAR(500) NOT NULL | |
| version | VARCHAR(100) NOT NULL | |
| publisher | VARCHAR(255) | nullable — บางตัวไม่มี |
| install_date | DATE | |
| install_path | TEXT | |
| install_size_kb | BIGINT | |
| arch | VARCHAR(10) | `x86`/`x64`/`arm64` |
| source | VARCHAR(20) NOT NULL | `registry`/`appstore`/`plist` |
| signature_id | BIGINT FK → signature_records.id NULL | |
| first_seen_scan_id | BIGINT FK → scans.id ON DELETE SET NULL | |
| last_seen_scan_id | BIGINT FK → scans.id ON DELETE SET NULL | |
| uninstalled_at | TIMESTAMPTZ NULL | set เมื่อ scan ไม่เจออีก |
| created_at | TIMESTAMPTZ | |
| updated_at | TIMESTAMPTZ | |

**Indexes:**
- `(machine_id, name, version)` UNIQUE (a software with same name+version on same machine = 1 row)
- `(name, version)` for cross-machine queries
- GIN trigram on `name` for fuzzy search

---

### `software_history`
Audit ของ install/uninstall events

| Column | Type | Notes |
|--------|------|-------|
| id | BIGSERIAL PK | |
| machine_id | BIGINT FK | |
| software_name | VARCHAR(500) NOT NULL | |
| software_version | VARCHAR(100) NOT NULL | |
| event | VARCHAR(20) NOT NULL | `installed` / `uninstalled` / `updated` |
| previous_version | VARCHAR(100) NULL | สำหรับ event=updated |
| occurred_at | TIMESTAMPTZ NOT NULL | |
| scan_id | BIGINT FK → scans.id | |

**Indexes:** `(machine_id, occurred_at DESC)`

---

### `signature_records`
Digital signature ของ executable ที่ scan เจอ

| Column | Type | Notes |
|--------|------|-------|
| id | BIGSERIAL PK | |
| uuid | UUID UNIQUE NOT NULL | |
| software_id | BIGINT FK → software_records.id ON DELETE CASCADE | |
| status | VARCHAR(20) NOT NULL | `valid`/`expired`/`invalid`/`unsigned` |
| signer | VARCHAR(500) | CN of leaf cert |
| issuer | VARCHAR(500) | CN of issuer |
| cert_thumbprint | VARCHAR(64) | SHA256 hex |
| cert_valid_from | DATE | |
| cert_valid_to | DATE | |
| signature_algorithm | VARCHAR(50) | |
| chain | JSONB | array of {subject, issuer, valid_from, valid_to, thumbprint} |
| verified_at | TIMESTAMPTZ NOT NULL DEFAULT NOW() | |

**Indexes:** `software_id`, `status`

---

### `cve_records`
Cache ของ CVE data จาก NVD/OSV

| Column | Type | Notes |
|--------|------|-------|
| id | BIGSERIAL PK | |
| cve_id | VARCHAR(20) UNIQUE NOT NULL | e.g. `CVE-2024-12345` |
| source | VARCHAR(10) NOT NULL | `nvd` / `osv` |
| published_at | TIMESTAMPTZ NOT NULL | |
| modified_at | TIMESTAMPTZ NOT NULL | |
| severity | VARCHAR(10) NOT NULL | `critical`/`high`/`medium`/`low` |
| cvss_score | NUMERIC(3,1) | |
| description | TEXT NOT NULL | |
| cpe_matches | JSONB NOT NULL | array of CPE strings |
| affected | JSONB | array of {product, vendor, version_range} |
| references | JSONB | array of URLs |
| created_at | TIMESTAMPTZ | |
| updated_at | TIMESTAMPTZ | |

**Indexes:** `cve_id` (unique), `severity`, `modified_at DESC`, GIN on `cpe_matches`

---

### `vulnerabilities`
Join table: ซึ่ง software มี CVE อะไรบ้าง

| Column | Type | Notes |
|--------|------|-------|
| id | BIGSERIAL PK | |
| software_id | BIGINT FK → software_records.id ON DELETE CASCADE | |
| cve_id | VARCHAR(20) FK → cve_records.cve_id ON DELETE CASCADE | |
| matched_at | TIMESTAMPTZ NOT NULL DEFAULT NOW() | |
| recommended_version | VARCHAR(100) | fixed-in version |
| is_dismissed | BOOLEAN NOT NULL DEFAULT FALSE | admin dismiss |
| dismissed_by_user_id | BIGINT FK → users.id NULL | |
| dismissed_reason | TEXT | |

**Indexes:** `(software_id, cve_id)` UNIQUE, `cve_id`, `(is_dismissed, matched_at DESC)`

---

### `whitelist_entries`
Software ที่อนุญาตให้ติดตั้ง

| Column | Type | Notes |
|--------|------|-------|
| id | BIGSERIAL PK | |
| uuid | UUID UNIQUE | |
| name_pattern | VARCHAR(500) NOT NULL | exact หรือ wildcard เช่น `Microsoft Office %` |
| publisher_pattern | VARCHAR(255) | nullable |
| version_constraint | VARCHAR(50) | semver-style เช่น `>=2.0` |
| notes | TEXT | |
| created_by_user_id | BIGINT FK → users.id | |
| created_at | TIMESTAMPTZ | |

**Index:** name_pattern (GIN trigram)

---

### `blacklist_entries`
Software ที่ห้ามติดตั้ง — schema เหมือน whitelist + severity

| Column | Type | Notes |
|--------|------|-------|
| (เหมือน whitelist) | | |
| severity | VARCHAR(10) NOT NULL DEFAULT 'high' | `high`/`medium`/`low` — กำหนด alert priority |
| reason | TEXT NOT NULL | ทำไมห้าม |

---

### `alerts`
Alert จาก detection (unauthorized software, etc.)

| Column | Type | Notes |
|--------|------|-------|
| id | BIGSERIAL PK | |
| uuid | UUID UNIQUE | |
| type | VARCHAR(30) NOT NULL | `unauthorized_software` / `blacklisted_software` / `signature_invalid` / `cve_critical` / `license_expiring` / `license_overused` |
| severity | VARCHAR(10) NOT NULL | |
| machine_id | BIGINT FK → machines.id ON DELETE CASCADE NULL | |
| software_id | BIGINT FK → software_records.id ON DELETE CASCADE NULL | |
| title | VARCHAR(500) NOT NULL | |
| details | JSONB NOT NULL | type-specific payload |
| status | VARCHAR(20) NOT NULL DEFAULT 'open' | `open`/`acknowledged`/`resolved` |
| created_at | TIMESTAMPTZ NOT NULL DEFAULT NOW() | |
| acknowledged_by_user_id | BIGINT FK → users.id NULL | |
| acknowledged_at | TIMESTAMPTZ NULL | |
| resolved_at | TIMESTAMPTZ NULL | |

**Indexes:** `(status, created_at DESC)`, `(machine_id, status)`, `type`

---

### `licenses`
License ที่บริษัทซื้อ

| Column | Type | Notes |
|--------|------|-------|
| id | BIGSERIAL PK | |
| uuid | UUID UNIQUE | |
| software_name | VARCHAR(500) NOT NULL | |
| publisher | VARCHAR(255) | |
| license_key | VARCHAR(500) | encrypted at rest (column-level encrypt) |
| purchased_count | INTEGER NOT NULL | |
| purchased_at | DATE | |
| expires_at | DATE NULL | NULL = perpetual |
| cost_total | NUMERIC(12,2) | |
| currency | VARCHAR(3) DEFAULT 'THB' | |
| vendor_contact | VARCHAR(255) | |
| notes | TEXT | |
| created_by_user_id | BIGINT FK → users.id | |
| created_at | TIMESTAMPTZ | |
| updated_at | TIMESTAMPTZ | |

**Indexes:** `software_name`, `expires_at`

---

### `license_installations`
Computed view ของ "license นี้ ติดตั้งบนเครื่องไหนบ้าง" — derived from software_records WHERE name matches.

> Materialized view, refresh ทุก scan-batch หรือ on-demand

```sql
CREATE MATERIALIZED VIEW license_installations AS
SELECT
  l.id AS license_id,
  sr.id AS software_id,
  sr.machine_id
FROM licenses l
JOIN software_records sr ON sr.name ILIKE l.software_name
  AND sr.uninstalled_at IS NULL
WHERE l.expires_at IS NULL OR l.expires_at > CURRENT_DATE;
```

---

### `audit_logs`
ทุก mutation ใน admin operations

| Column | Type | Notes |
|--------|------|-------|
| id | BIGSERIAL PK | |
| user_id | BIGINT FK → users.id NULL | NULL = system |
| action | VARCHAR(50) NOT NULL | e.g. `user.create`, `whitelist.add`, `license.delete` |
| entity_type | VARCHAR(50) NOT NULL | |
| entity_id | VARCHAR(50) | uuid หรือ id |
| changes | JSONB | {before: ..., after: ...} |
| ip_address | INET | |
| user_agent | TEXT | |
| created_at | TIMESTAMPTZ NOT NULL DEFAULT NOW() | |

**Indexes:** `(user_id, created_at DESC)`, `(entity_type, entity_id)`, `created_at DESC`

> Partition by month, retain 1 year

---

## Migrations Strategy

- 1 migration per feature/PR
- ห้าม edit migration ที่ apply แล้ว — เขียน migration ใหม่
- Migration ต้อง reversible (`def downgrade()` ต้องเขียน)
- ไม่ใช้ `alembic auto-generate` blindly — review ก่อนเสมอ

## Initial Seed Data

หลัง migration แรก, รัน seed script สร้าง:
- 1 admin user (email + password จาก env var)
- 0 machines, 0 software (cold start)

## Query Performance Notes

- `software_records` จะใหญ่ที่สุด — 1000 machines × 500 software = 500,000 rows. Index ต้องครบ
- `cve_records` จาก NVD ปัจจุบันมี ~250,000 entries. ใช้ `JSONB + GIN` สำหรับ cpe_matches
- `audit_logs` partition by month เพื่อ drop เก่าง่าย
- `heartbeats` partition by week, retain 7 days
