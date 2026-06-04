# Module 6 — License Compliance

## Purpose

- เก็บข้อมูล license ที่บริษัทซื้อ
- เทียบ purchased vs installed count → over/under license
- แจ้งเตือนก่อน license หมดอายุ (30/60/90 วัน)
- Compliance rate widget

## Phase
Phase 4

## Depends on
- Module 2 (Software Inventory)

---

## Features & Acceptance Criteria

### 6.1 License CRUD
**UI:** `/licenses`

**Form fields:**
- software_name (required) — ตรงกับ name pattern ของ software inventory
- publisher (optional)
- license_key (optional, encrypted at rest)
- purchased_count (required, integer ≥ 1)
- purchased_at (optional, date)
- expires_at (optional, date — NULL = perpetual)
- cost_total (optional, decimal)
- currency (default THB)
- vendor_contact (optional, email/phone)
- notes (optional, free text)

**Actions:** Create, Read, Update, Delete (admin only)

**Acceptance:**
- Add license "Microsoft Office", purchased=20, expires=2026-12-31
- license_key encrypted ใน DB (column-level AES)
- Edit เก็บ history ผ่าน audit_log

### 6.2 Installation count (auto)
**Computed view:**
```sql
SELECT
    l.id AS license_id,
    COUNT(DISTINCT sr.machine_id) AS installed_count
FROM licenses l
LEFT JOIN software_records sr
    ON sr.name ILIKE l.software_name
    AND sr.uninstalled_at IS NULL
GROUP BY l.id;
```

Refresh:
- After every scan ingestion (background job)
- หรือ on-demand (button "Refresh" บน license page)
- Cache 5 min ใน Redis

**Acceptance:**
- License "Adobe Photoshop", 5 เครื่องติดตั้ง → installed_count=5
- ติดตั้งเครื่องที่ 6 → next scan → installed_count=6
- Uninstall → installed_count ลด

### 6.3 Over-licensed / Under-licensed flag
**Computed status:**
- `compliant` — installed ≤ purchased AND not expired
- `over_used` — installed > purchased
- `expired` — expires_at < today
- `expiring_soon` — expires_at ≤ today + 90 days (sub-class: 30/60/90)

**Display:** colored badge ใน license table

**Acceptance:**
- License 20 ซื้อ, 25 ติดตั้ง → status=over_used, badge red
- License expire ใน 25 วัน → status=expiring_soon (30d), badge orange
- License expire เมื่อวาน → status=expired, badge red

### 6.4 Expiry alerts (30/60/90 days)
**Worker:** daily job `license_expiry_check`

**Logic:**
```python
today = date.today()
for license in licenses:
    if license.expires_at is None: continue
    days_until = (license.expires_at - today).days
    if days_until in (90, 60, 30, 14, 7, 0):
        emit_alert(type='license_expiring', severity=severity_for_days(days_until), ...)
    if days_until < 0:
        emit_alert(type='license_expired', severity='high', ...)
```

**Dedup:** per (license_id, milestone_days) — 1 alert per milestone

**Acceptance:**
- License expire ใน 30 วัน → alert ที่ severity=medium
- 7 วัน → high
- หมดอายุแล้ว → critical
- Renew license (update expires_at) → ไม่ alert ซ้ำสำหรับ milestone เก่า

### 6.5 Compliance summary widget (Dashboard)
**KPI card:** compliance rate = `compliant / total`

**Detail:**
- Total licenses
- Compliant (green)
- Over-used (red)
- Expiring 30/60/90 (yellow gradient)
- Expired (gray)

**Acceptance:**
- 18/23 compliant → rate = 78%
- Click → drill to license page

### 6.6 Bulk import licenses
**Optional v1, target Phase 4 ถ้าเวลาพอ**

**CSV format:**
```csv
software_name,publisher,purchased_count,purchased_at,expires_at,cost_total,currency,notes
Microsoft Office,Microsoft,50,2024-01-01,2026-12-31,250000,THB,...
```

**Validation:** integer, date format, etc.

### 6.7 License → installed machines drill-down
**UI:** Click license row → modal show list of machines ที่ติดตั้ง

**Action:**
- Export machine list (CSV)
- Compare against allowed-users list (manually maintained for now)

**Acceptance:**
- Click "Adobe Photoshop" license → modal show 7 เครื่อง (if installed_count=7)
- Export → CSV download

---

## API endpoints

```
GET    /api/v1/licenses
GET    /api/v1/licenses/{uuid}
POST   /api/v1/licenses
PATCH  /api/v1/licenses/{uuid}
DELETE /api/v1/licenses/{uuid}
POST   /api/v1/licenses/bulk           # CSV import
GET    /api/v1/licenses/{uuid}/installations   # list machines
GET    /api/v1/licenses/compliance-summary
POST   /api/v1/licenses/refresh-counts  # manual recompute
```

---

## DB tables

- `licenses`
- `license_installations` (materialized view)
- `alerts` (type `license_expiring`, `license_expired`, `license_overused`)
- License key encryption: column-level via `cryptography.fernet`

---

## Encryption pattern (license_key)

```python
from cryptography.fernet import Fernet
import os

_fernet = Fernet(os.environ['LICENSE_KEY_ENCRYPTION_KEY'].encode())

def encrypt(plain: str | None) -> bytes | None:
    return _fernet.encrypt(plain.encode()) if plain else None

def decrypt(cipher: bytes | None) -> str | None:
    return _fernet.decrypt(cipher).decode() if cipher else None
```

SQLAlchemy TypeDecorator wrap:
```python
class EncryptedString(TypeDecorator):
    impl = LargeBinary
    cache_ok = True
    def process_bind_param(self, value, _): return encrypt(value)
    def process_result_value(self, value, _): return decrypt(value)
```

---

## UI components

| Component | Notes |
|-----------|-------|
| `LicenseTable` | sortable, with status badges |
| `LicenseForm` | add/edit modal |
| `LicenseStatusBadge` | compliant/over/expired/expiring |
| `ComplianceSummaryWidget` | KPI card + breakdown |
| `LicenseInstallationsDrawer` | side drawer with machine list |
| `LicenseKeyField` | secret input (toggle show/hide) |

---

## Edge cases

1. **software_name pattern matches multiple SKUs** (e.g. "Microsoft Office %" matches Office 2019 + Office 365): installation count รวมหมด — admin ใช้ exact pattern ถ้าต้องการแยก
2. **License "perpetual"** (`expires_at = NULL`): never alert expiry — only over-use
3. **Renewal**: edit license + update `expires_at` → milestone alerts reset
4. **License_key มีพิเศษ char**: encrypt handle binary-safe (Fernet returns base64)
5. **Same software ติดตั้ง 2 versions บนเครื่องเดียว** (Office 2019 + Office 365): นับเป็น 1 installation per machine (DISTINCT machine_id)
6. **Soft-deleted machine**: ไม่นับใน installed_count
7. **Over-used แต่ admin ยอมรับ**: ใน v1 ยัง flag ตลอด — v2 อาจมี "exception" feature

---

## Test plan

### Unit
- Encryption round-trip
- Days-until milestone bucket
- Status calculation (compliant/over/expired)

### Integration
- POST license + scan with installations → installed_count correct
- Add 6th installation → status=over_used
- Set expires_at to 25 days from now → daily job → expiring_soon alert
- Update expires_at → milestone reset

### Frontend
- License form full flow
- Status badge color/text correct per status
- Drill-down drawer shows machines correctly

### Manual QA
- Create license แล้วลบ → audit_log records
- Encrypt key visible ใน DB เป็น ciphertext
- Compliance widget updates after scan
