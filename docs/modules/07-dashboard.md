# Module 7 — Dashboard

## Purpose

หน้า landing สำหรับ admin/viewer — แสดงภาพรวม risk + ทาง drill-down ไปทุก module

## Phase
Phase 4

## Depends on
- Modules 1-6 (ทั้งหมด — dashboard aggregate ของทุกอย่าง)

---

## Pages

### 7.1 Overview (root `/`)
Layout:
```
┌─────────────────────────────────────────────────────────────┐
│   Machines: 1,234        Agents online: 1,100 / 1,234       │
│   Software (unique): 4,567   Vulnerabilities: 5C/23H/87M    │
└─────────────────────────────────────────────────────────────┘
┌──────────────────┬──────────────────┬──────────────────┐
│ Vuln Severity    │ Sig Status       │ License          │
│  (donut)         │  (donut)         │  (gauge 78%)     │
└──────────────────┴──────────────────┴──────────────────┘
┌─────────────────────────────────────────────────────────────┐
│ Vulnerability trend (last 30 days) — line chart             │
└─────────────────────────────────────────────────────────────┘
┌──────────────────────────────┬──────────────────────────────┐
│ Top 10 Risky Machines        │ Real-time Alert Feed         │
│  bar (risk score)            │  list, paginate, ack/resolve │
└──────────────────────────────┴──────────────────────────────┘
```

**KPI cards (top row):**
- Total machines (link → `/machines`)
- Agents online / total (with offline alert if % > 10)
- Total software unique (link → `/software`)
- Vulnerabilities by severity (link → `/vulnerabilities`)

**Charts (middle row):** Donut + gauge — click segment → filtered drill-down

**Bottom row:**
- Top 10 risky machines (bar chart, sorted desc by risk score)
- Alert feed (last 10 open alerts, type+severity+timestamp)

**Auto-refresh:** poll `/api/v1/dashboard/overview` ทุก 30s. ผู้ใช้ toggle pause ได้

**Acceptance:**
- Overview load ≤ 2s ที่ fleet 1000 machines
- Donut/bar render ถูกตัวเลข
- Click drill-down → ไปหน้าถูก + filter ติด
- Refresh button + auto-poll ทำงาน

### 7.2 Per-machine detail (`/machines/{uuid}`)
Tabs:
- **Overview**: risk score, online status, OS, last scan, tags
- **Software** (from Module 2)
- **Vulnerabilities** (from Module 5)
- **Signatures** (filter signed/unsigned)
- **History** (install/uninstall events)
- **Alerts** (all alerts for this machine)

**Header:**
- Hostname + status indicator (green/yellow/red dot)
- Action buttons: "Trigger scan", "Edit tags", "Delete" (admin)

**Risk score card:**
- Number + color + breakdown:
  - Unsigned: N (×1)
  - Unauthorized: N (×3)
  - CVE Critical: N (×5)
  - CVE High: N (×3)
  - CVE Medium: N (×1)
- Trend sparkline (last 7 days)

**Acceptance:**
- Hostname update real-time (poll 30s)
- Tab switch ไม่ refetch ที่ไม่จำเป็น (cached via React Query)
- Trigger scan button → toast + flag set + status badge → "Scan requested"

---

## Risk Score Formula

```
risk_score = (unsigned_count × 1)
           + (unauthorized_count × 3)
           + (cve_critical × 5)
           + (cve_high × 3)
           + (cve_medium × 1)
           + (cve_low × 0.5)
```

**Color thresholds:**
- 0 — green
- 1-10 — yellow
- 11-30 — orange
- 31+ — red

Configurable ใน `policy_settings.risk_thresholds` (v2; v1 hardcode)

**Acceptance:**
- เครื่องไม่มี issue → score 0, green
- 1 CVE critical → score 5, yellow
- 10 unauthorized + 2 critical CVE → score 40, red

---

## Real-time Alert Feed

**Source:** `GET /api/v1/alerts?status=open&limit=10&sort=created_at:desc`

**Polling:** ทุก 30s
**Future:** WebSocket / SSE สำหรับ true real-time (v2)

**Per alert item:**
- Severity badge (color)
- Type + title
- Machine link
- Time ("2m ago")
- Quick actions: Acknowledge / Resolve / Dismiss (admin)

**Acceptance:**
- New alert appears within 30s
- Click machine link → drill
- Acknowledge → status updates immediately (optimistic) + refetch

---

## Charts (Recharts)

### Vuln trend (line)
- X-axis: date (last 30 days)
- Y-axis: count
- 4 lines: Critical, High, Medium, Low
- Color: red, orange, yellow, gray
- Hover tooltip: exact count
- Empty days = 0

### Signature status (donut)
- 4 segments: Valid, Expired, Invalid, Unsigned
- Colors: green, amber, red, gray
- Center label: total count
- Click → drill

### License compliance (gauge)
- 0-100% with color zones
- Green ≥ 90%, yellow 70-90%, red < 70%

### Top 10 risky machines (horizontal bar)
- X-axis: risk score
- Y-axis: hostname
- Color: gradient red intensity
- Click bar → machine detail

---

## API endpoints

```
GET /api/v1/dashboard/overview              # KPIs + counts
GET /api/v1/dashboard/risk-scores           # ?limit=10
GET /api/v1/dashboard/charts/vuln-trend     # ?period=30d
GET /api/v1/dashboard/charts/signature      # status counts
GET /api/v1/dashboard/charts/license        # compliance bucket counts
GET /api/v1/alerts                           # alerts feed (shared)
```

---

## UI components

| Component | Notes |
|-----------|-------|
| `KPICard` | metric + delta + drilldown link |
| `RiskScoreCard` | breakdown + sparkline |
| `VulnTrendChart` | recharts LineChart |
| `SignatureDonut` | PieChart |
| `LicenseGauge` | custom Radial chart |
| `RiskyMachinesBar` | horizontal BarChart |
| `AlertFeed` | virtualized list, real-time |
| `RefreshControl` | pause/resume auto-refresh |

---

## Performance considerations

- Overview endpoint = aggregate query → cache result ใน Redis (TTL 30s)
- Risky machines = compute `risk_score` ที่ ingest time, store `machines.risk_score` column → query super fast
- Trend chart = group by date → pre-aggregate ใน `vuln_daily_counts` table refreshed nightly

---

## Edge cases

1. **Empty fleet (0 machines)**: dashboard shows empty state with onboarding link
2. **All agents offline**: KPI banner "All agents offline. Check network/agent service"
3. **0 vulnerabilities**: chart hidden หรือ show "No vulnerabilities! 🎉" (no emoji ใน production unless user asks; use copy)
4. **Massive alert spike**: feed limit 10, link "View all" → full alerts page
5. **Slow network**: skeleton loaders + partial render (ทำได้ทีละ widget)

---

## Test plan

### Unit
- Risk score formula
- Color threshold mapping
- Date bucketing (vuln trend)

### Integration
- Seed data → overview endpoint returns correct counts
- Update machine vulns → risk_score recomputed
- Alert created → feed includes within next poll

### Frontend
- Loading states (skeleton)
- Empty states
- Error states (API down → friendly message + retry)
- Drill-down navigation preserves filters
- Auto-refresh pauses on user interaction (scroll, hover) — TBD UX detail

### Visual regression
- Charts render consistent across browsers (Chrome, Safari, Firefox)
- Dark mode (Phase 5)
