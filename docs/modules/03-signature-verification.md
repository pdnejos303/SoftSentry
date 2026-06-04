# Module 3 — Digital Signature Verification

## Purpose

แสดง + วิเคราะห์ digital signature ของ software:
- Status: Signed (valid) / Signed (expired) / Signed (invalid) / Unsigned
- Certificate chain detail
- Auto-flag unsigned/invalid → alert

## Phase
Phase 3 (UI). ฝั่ง agent ทำเสร็จใน Phase 2 ตาม [Module 1](01-agent.md)

## Depends on
- Module 1 (agent ส่ง signature data ขึ้นมา)
- Module 2 (software records linked to signature)

---

## Features & Acceptance Criteria

### 3.1 Display signature status
**UI:** ใน software list table → column "Signature" with colored badge

**Badge colors:**
- 🟢 Valid — green
- 🟡 Expired — amber
- 🔴 Invalid — red
- ⚫ Unsigned — gray
- ❓ Unknown — outline (verify failed e.g. file missing)

**Acceptance:**
- ทุก software มี badge
- Hover → tooltip show signer + valid_to date
- Click → modal show full chain (next feature)

### 3.2 Certificate chain detail
**UI:** Modal/drawer เปิดจากคลิก badge

**Sections:**
- **Status**: large colored indicator + text
- **Leaf cert**: signer CN, valid from/to, thumbprint, sig algorithm
- **Chain**: tree view (leaf → intermediate(s) → root), แต่ละ node แสดง subject + issuer + valid period
- **Issues**: ถ้า status ≠ valid → bullet list ของปัญหา (e.g. "Cert expired 2 months ago", "Self-signed", "Untrusted root")

**Acceptance:**
- Chain ของ Microsoft Office → 3-tier chain แสดงครบ
- Self-signed software → flag "Untrusted root" ใน issues
- Expired cert → calculate "expired N days ago"

### 3.3 Filter "Unsigned only" / by status
**UI:** Filter dropdown ใน software list (both per-machine และ cross-machine)

**Acceptance:**
- Select "Unsigned" → list filter ถึงเฉพาะ unsigned
- Multi-select: Unsigned + Invalid → union

### 3.4 Auto-flag unsigned/invalid → alert
**Worker:** alert engine processes new scan → emit alert
- Type `signature_invalid` for status=invalid
- Type `signature_unsigned` for status=unsigned (configurable: admin อาจปิดถ้า noisy)

**Severity rule:**
- Invalid → high
- Unsigned → medium (default — configurable)
- Expired → medium

**Dedup:**
- Per (machine_id, software_id, type) — 1 alert ค้างได้ครั้งเดียว, แก้แล้วปิด → alert ใหม่ถ้ามี issue กลับมา

**Acceptance:**
- ติดตั้ง .exe ที่ไม่ signed → alert ขึ้นภายใน 1 scan cycle
- ผู้ใช้ uninstall → next scan → ตรวจไม่เจอ → alert resolve อัตโนมัติ
- Software เดิม alert เปิดอยู่ → next scan ยัง unsigned → ไม่สร้าง alert ใหม่

### 3.5 Signature statistics widget (Dashboard)
**Pie/donut chart:** distribution ของ signature status ทั่วทั้ง fleet

**Acceptance:**
- แสดง 4 segments + count + percentage
- Click segment → drill to filtered software list

### 3.6 Whitelist trusted publishers (optional v1)
**Use case:** organization ใช้ internal software signed by company CA → ต้องไม่ flag เป็น invalid

**UI:** `/settings/trusted-publishers`
- Add publisher CN + cert thumbprint
- ถ้า software's signer/thumbprint อยู่ใน list → override status เป็น `valid`

**Acceptance:**
- Add "Mycompany Internal CA" → all software signed by it shows as valid
- Remove from list → status revert

> **หมายเหตุ:** ถ้า scope ตึง อาจเลื่อน feature นี้ไป v2

---

## Data flow

```
Agent verify signature → embedded in scan POST payload
Backend ingest:
  - Insert/update signature_records
  - Link software_records.signature_id
  - Enqueue: check_signature_alerts(software_id)
Worker check_signature_alerts:
  - Read signature_records.status
  - Apply trusted_publishers override (ถ้ามี)
  - Create alert ถ้าสมควร
```

---

## API endpoints

- `GET /api/v1/signatures` — filter + paginated (see [API contracts](../04-api-contracts.md))
- `GET /api/v1/signatures/{software_uuid}` — full chain detail
- `GET /api/v1/dashboard/signature-stats` — donut chart data
- `GET|POST|DELETE /api/v1/trusted-publishers` (ถ้า implement 3.6)

---

## DB tables

- `signature_records` — main
- `software_records.signature_id` FK
- `alerts` — type `signature_invalid` / `signature_unsigned`
- `trusted_publishers` (optional)

---

## UI components

| Component | Notes |
|-----------|-------|
| `SignatureBadge` | colored badge with tooltip |
| `SignatureDetailModal` | full chain view |
| `CertChainTree` | tree visual ของ chain |
| `SignatureStatsWidget` | donut chart |
| `SignatureStatusFilter` | multi-select dropdown |

---

## Edge cases

1. **File ที่ verify ไม่ได้** (locked, in use, missing): status = `unknown`, error_reason field log ไว้
2. **Software ไม่มี executable หลัก** (data-only, e.g. fonts pkg): signature_id = NULL, ไม่ flag
3. **Cross-signed cert** (Microsoft + DigiCert): pick chain ที่ verify สำเร็จ
4. **Cert reissued** (new thumbprint, same subject): ถือเป็น cert ใหม่ — เก็บประวัติทั้งคู่ผ่าน verified_at
5. **System time wrong**: agent verify ใช้ system time → false expired. Mitigation: backend re-evaluate based on `cert_valid_to` vs server time when displaying

---

## Test plan

### Backend unit
- Mapping function: `WinVerifyTrust` return codes → status enum
- Trusted publisher override logic
- Alert dedup logic

### Backend integration
- POST scan with signature_status=unsigned → alert created
- Same scan again → no duplicate alert
- POST scan ที่ status เปลี่ยน unsigned → valid → resolve old alert

### Frontend
- Badge renders correct color per status
- Modal opens + shows chain
- Filter works (network call with correct query)

### Manual QA
- Verify signature ของ Microsoft Office (multi-tier chain) → valid
- Old version of software มี cert expired → expired
- File ที่ใช้ self-signed → invalid (untrusted root)
- Notepad (built-in Windows) → signed by Microsoft Windows Production
