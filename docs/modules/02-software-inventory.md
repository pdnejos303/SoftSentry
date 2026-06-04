# Module 2 — Software Inventory

## Purpose

แสดงรายการ software ที่ติดตั้งบนทุกเครื่อง — search, filter, compare, history

## Phase
Phase 2

## Depends on
- Module 1 (Agent) — แหล่งข้อมูล

---

## Features & Acceptance Criteria

### 2.1 List software ต่อเครื่อง
**UI page:** `/machines/{uuid}` → tab "Software"

**Features:**
- Table: name, version, publisher, install_date, signature status, vuln count, install_size
- Search box (debounced 300ms, search by name + publisher)
- Filter: signature_status (multi-select), has_vulnerability (yes/no), publisher
- Sort: any column
- Pagination 50/page

**Acceptance:**
- เครื่องมี 500 software → load ได้ ≤ 1s
- Search "chrome" → return Chrome + Chrome Helper เป็นต้น
- Filter signature=unsigned → ตรงทุก row

### 2.2 Cross-machine search
**UI page:** `/software`

**Features:**
- Aggregate view — 1 row per unique `(name, version)`
- Column: name, version, publisher, **installed_count**, signature_status (worst-case), vulnerabilities
- Click name → drill to detail page show list of machines
- Search same as per-machine

**Acceptance:**
- 1000 machines × ติดตั้ง Chrome พร้อมกัน → 1 row, installed_count=1000
- คลิก row → list 1000 machines (paginated)

### 2.3 Filter by name/publisher/version
- Combine filters AND
- Free-text `q` search ใน name + publisher (Postgres FTS หรือ trigram)
- Filter chips ลบได้รายตัว

**Acceptance:**
- Filter "Adobe" + version "*2024*" → ตรงเฉพาะ Adobe products version contain "2024"

### 2.4 Compare 2 machines
**UI page:** `/machines/compare?a=<uuid>&b=<uuid>` (เลือกจาก machine list, action "compare with...")

**Result sections:**
- Common software
- Only in A
- Only in B
- Different version (same name, different version)

**Acceptance:**
- Compare 2 เครื่อง A มี 100 software, B มี 80 software, ซ้ำ 60 → common=60, only_a=40, only_b=20
- Different version: เครื่อง A มี Chrome 119, B มี Chrome 120 → show ใน "version diff"

### 2.5 Software history
**UI:** ใน machine detail page → tab "History"

**Events:**
- Software installed (new in this scan, not in previous)
- Software uninstalled (in previous scan, not in this)
- Software updated (same name, different version)

**Computed:** Backend ดู `software_history` table (populated ตอน process scan)

**Acceptance:**
- ติดตั้ง software ใหม่ → next scan → event `installed` ขึ้นใน history
- Uninstall → next scan → event `uninstalled`
- Update version → event `updated` with previous_version

### 2.6 Top 10 software
**UI:** dashboard widget + page `/software?sort=installed_count:desc&page_size=10`

**Acceptance:**
- Widget แสดงสูงสุด 10, bar chart visual
- Click → ไปหน้า list filtered

---

## Data flow (backend)

### Scan ingestion
```
POST /api/v1/agents/scans  →
  1. Validate agent token
  2. Insert into scans table
  3. For each software in payload:
     - Upsert software_records (key: machine_id, name, version)
     - Update last_seen_scan_id
     - Upsert signature_records linked to software
  4. Mark software_records ที่ไม่อยู่ใน scan นี้ + last_seen_scan_id < this scan → uninstalled_at = NOW()
  5. Insert software_history events (install/update/uninstall)
  6. Enqueue jobs: match_alerts, match_cve, refresh_license_view
```

ทำใน background task (FastAPI BackgroundTasks หรือ arq เพื่อ idempotent retry)

---

## API endpoints
ดูเต็มที่ [API contracts](../04-api-contracts.md). ที่เกี่ยวข้องกับ module นี้:

- `GET /api/v1/machines/{uuid}/software`
- `GET /api/v1/machines/{uuid}/history`
- `GET /api/v1/software` (cross-machine aggregate)
- `GET /api/v1/software/top`
- `POST /api/v1/software/compare`

---

## DB tables touched
- `software_records` (heavy write)
- `software_history`
- `signature_records`

ดู schema ที่ [Data Model](../03-data-model.md)

---

## UI components needed

| Component | Notes |
|-----------|-------|
| `SoftwareTable` | shadcn DataTable + TanStack Table |
| `SoftwareFilters` | filter chips bar |
| `SoftwareSearchInput` | debounced search |
| `SoftwareCompareView` | 3-column layout (only A / common / only B + version diff) |
| `HistoryTimeline` | vertical timeline จาก software_history |
| `TopSoftwareWidget` | bar chart (recharts) |

---

## Performance considerations

- 1000 machines × 500 software = **500,000 rows** ใน `software_records`
- Index ที่ต้องมี:
  - `(name, version)` — สำหรับ cross-machine
  - `(machine_id, uninstalled_at)` — สำหรับ per-machine list
  - GIN trigram on `name` — สำหรับ ILIKE search
- Aggregate query (`/software` cross-machine): pre-aggregate ใน materialized view ถ้า slow (วัดก่อนทำ)
- Pagination บังคับ — ห้าม return ทุก row

---

## Edge cases

1. **Software ใน scan แต่ไม่ใน previous → installed event**: บางที agent restart → scan สอง record ซ้อน 5 นาที. ป้องกัน: idempotency_key + dedupe ที่ backend
2. **Massive scan (1000+ software)**: split ที่ agent ไม่จำเป็นใน v1; backend bulk insert ด้วย `INSERT ... ON CONFLICT DO UPDATE`
3. **Software name มี leading/trailing whitespace**: trim ก่อน upsert (registry บางทีมี space)
4. **Version `1.0.0.0` vs `1.0.0`**: เก็บแบบที่ agent ส่ง — ห้ามแก้. แต่ search/compare ใช้ semver normalize ที่ search-time
5. **เครื่องถูกลบ (soft delete)**: software records ของเครื่องนั้นไม่ปรากฏใน aggregate (filter `WHERE deleted_at IS NULL`)

---

## Test plan

### Backend integration
- Submit scan → assert software_records updated
- Submit scan ที่ขาด software เดิม → assert uninstalled_at set + history event
- Cross-machine query: 3 machines × overlapping software → assert installed_count
- Compare endpoint: ตรง expected partition

### Frontend
- Table renders + paginate
- Search debounce works (no fetch per keystroke)
- Filter chips remove → query updates
- Compare page handles empty result

### Performance
- Seed 1000 machines × 500 software → page load ≤ 2s
- Search "a%" (broad) ≤ 1s
