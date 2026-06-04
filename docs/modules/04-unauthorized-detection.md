# Module 4 — Unauthorized Software Detection

## Purpose

Whitelist + Blacklist policy:
- Whitelist: software ที่อนุญาต — software นอกรายการ = flag
- Blacklist: software ที่ห้าม — เจอเมื่อไหร่ alert ทันที
- Bulk import จาก CSV
- Alert ส่งให้ admin

## Phase
Phase 3

## Depends on
- Module 2 (Software Inventory)

---

## Two modes

| Mode | Behavior |
|------|----------|
| **Blacklist only** (default) | Alert เมื่อเจอ software ที่อยู่ใน blacklist. ที่เหลือ allow ทั้งหมด |
| **Whitelist enforcement** | Alert เมื่อเจอ software **ไม่อยู่ใน** whitelist (mode strict สำหรับ org ที่ควบคุมเข้มงวด) |

Mode toggle ที่ `/settings/policy` (admin only)

---

## Features & Acceptance Criteria

### 4.1 Whitelist CRUD
**UI:** `/policy/whitelist`

**Entry fields:**
- name_pattern (required) — exact หรือ SQL LIKE wildcard (`%`, `_`), e.g. `Microsoft Office %`
- publisher_pattern (optional)
- version_constraint (optional) — semver-style เช่น `>=2.0,<3.0`
- notes (optional)

**Actions:** add, edit, delete (admin), search list

**Acceptance:**
- Add `Microsoft Office %` → match "Microsoft Office 2019", "Microsoft Office 365"
- Wildcard validation: `%` แทน 0+ chars, `_` แทน 1 char (SQL LIKE syntax)
- Delete → confirm dialog

### 4.2 Blacklist CRUD
**UI:** `/policy/blacklist`

**เหมือน whitelist + เพิ่ม:**
- severity (required, default `high`) — `high`/`medium`/`low`
- reason (required) — ทำไมห้าม

**Acceptance:**
- Add "uTorrent" + severity=high + reason="P2P prohibited"
- ทุก entry มี reason — UI บังคับ

### 4.3 Bulk import CSV
**UI:** Button "Bulk import" บน whitelist/blacklist page

**CSV format:**
```csv
name_pattern,publisher_pattern,version_constraint,notes
Microsoft Office %,Microsoft Corporation,,Standard office suite
Adobe Creative Cloud,Adobe Inc,,Design team
```

(Blacklist เพิ่ม `severity,reason`)

**Flow:**
1. Upload CSV (max 5MB)
2. Validate header
3. Parse + validate rows (show errors)
4. Confirm "Add 25 entries, skip 2"
5. Insert

**Acceptance:**
- Valid CSV 100 rows → all imported
- Row ที่ duplicate (same name_pattern + publisher_pattern) → skip + report
- Row ที่ malformed → error message ที่ row N, ไม่ใช่ generic error
- Atomic: ถ้า DB error กลางคัน → rollback ทั้งหมด

### 4.4 Auto-detect on scan
**Worker:** background job `match_unauthorized` triggered ทุกครั้งหลัง scan ingest

**Logic:**
```python
for sw in new_or_updated_software:
    # Blacklist check (always-on)
    if matches_any_pattern(sw, blacklist):
        create_alert(type='blacklisted_software', severity=entry.severity, ...)

    # Whitelist enforcement (if mode enabled)
    if policy.whitelist_mode and not matches_any_pattern(sw, whitelist):
        create_alert(type='unauthorized_software', severity='medium', ...)
```

**Acceptance:**
- Install software ที่อยู่ใน blacklist → alert ขึ้นภายใน 1 scan cycle
- Software ที่ matched whitelist (whitelist mode) → no alert
- Software ที่ไม่ matched whitelist (whitelist mode on) → alert
- Pattern wildcard work: blacklist `BitTorrent%` → match "BitTorrent 7.10"

### 4.5 Alert dedup + resolve
- Per (machine_id, software_id, type): 1 active alert
- เมื่อ scan ใหม่ + software ถูก uninstall → resolve alert
- Admin acknowledge → status `acknowledged` (ไม่หาย แต่ไม่ blink)
- Admin resolve manually → status `resolved`

**Acceptance:**
- Re-scan ครั้งที่ 2 → ไม่ดับเบิ้ล alert
- Uninstall offending software → alert auto-resolve
- ติดตั้งกลับมา → alert ใหม่

### 4.6 Notification (Phase 5 — extension)
- Email/Slack ส่งเมื่อมี alert severity=high ใหม่
- Config recipient per role / per channel
- Throttle: ถ้า > 10 alerts ใน 5 นาที → digest 1 ฉบับ

**v1 = in-app เท่านั้น** (Module 7 Dashboard real-time feed)

---

## Pattern matching

### name_pattern
- Use SQL `ILIKE` (case insensitive)
- Wildcard: `%` = 0+, `_` = exactly 1
- Plain text (no wildcard) → exact match (case-insensitive)

### publisher_pattern
- Same as name_pattern but on `software_records.publisher`
- NULL → ignore (don't filter by publisher)

### version_constraint
- Semver-style: `>=1.0,<2.0`, `^2.0`, `~1.4.0`
- Use `packaging.specifiers.SpecifierSet` (Python lib)
- Best-effort: ถ้า version ไม่ใช่ semver-compatible → match by default (don't filter out)

**Match function:**
```python
def matches_entry(sw: Software, entry: PolicyEntry) -> bool:
    if not ilike_match(sw.name, entry.name_pattern):
        return False
    if entry.publisher_pattern and not ilike_match(sw.publisher or '', entry.publisher_pattern):
        return False
    if entry.version_constraint:
        try:
            spec = SpecifierSet(entry.version_constraint)
            return Version(sw.version) in spec
        except InvalidVersion:
            return True   # version not parseable → don't filter
    return True
```

---

## API endpoints

```
# Whitelist
GET    /api/v1/whitelist
POST   /api/v1/whitelist
PATCH  /api/v1/whitelist/{uuid}
DELETE /api/v1/whitelist/{uuid}
POST   /api/v1/whitelist/bulk         # multipart CSV

# Blacklist (same shape)
GET    /api/v1/blacklist
POST   /api/v1/blacklist
PATCH  /api/v1/blacklist/{uuid}
DELETE /api/v1/blacklist/{uuid}
POST   /api/v1/blacklist/bulk

# Policy mode
GET    /api/v1/policy
PATCH  /api/v1/policy                 # toggle whitelist_mode

# Alerts (shared with other modules)
GET    /api/v1/alerts?type=unauthorized_software,blacklisted_software
POST   /api/v1/alerts/{uuid}/acknowledge
POST   /api/v1/alerts/{uuid}/resolve
```

---

## DB tables

- `whitelist_entries`
- `blacklist_entries`
- `policy_settings` (single row, `whitelist_mode bool`)
- `alerts`

---

## UI components

| Component | Notes |
|-----------|-------|
| `PolicyEntryTable` | reusable: whitelist + blacklist |
| `PolicyEntryForm` | add/edit modal |
| `BulkImportDialog` | file upload + preview + confirm |
| `PolicyModeToggle` | admin only |
| `AlertFeed` | shared with other modules, filterable |

---

## Edge cases

1. **Whitelist mode + empty whitelist**: ทุก software จะ alert. Show banner: "Whitelist mode is ON but list is empty. Add entries first."
2. **Pattern conflict** (whitelist + blacklist match กัน): blacklist ชนะ (more restrictive wins)
3. **Bulk import CSV ที่มี BOM**: handle UTF-8 BOM transparently
4. **Software ที่เคย unauthorized → admin add to whitelist**: existing alert ไม่ auto-resolve (admin ต้องกด resolve manually) — เพื่อ audit trail
5. **Performance**: 1000 machines × 500 software × 100 whitelist entries = 50M comparisons. Strategy:
   - Per scan แค่ delta (new/updated software) — ไม่ scan ทั้ง fleet
   - Index name_pattern as trigram
   - Run policy match ใน background job, ไม่ block API

---

## Test plan

### Unit
- Pattern matching (% wildcard, version constraint edge cases)
- CSV parser (BOM, quotes, escaped commas)
- Dedup logic

### Integration
- POST scan → blacklist hit → alert created
- POST scan → blacklist hit → POST same scan → no duplicate alert
- Whitelist mode ON → software ไม่ใน list → alert
- Bulk import valid + invalid rows → partial accept, errors reported
- Toggle policy mode → re-evaluate ที่ scan ถัดไป

### Frontend
- Table add/edit/delete works
- Bulk import dialog full flow
- Filter alerts by type
