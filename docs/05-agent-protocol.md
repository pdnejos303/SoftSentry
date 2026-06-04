# 05 — Agent ↔ Server Protocol

## Overview

Agent ใช้ HTTPS polling-based protocol (ไม่ใช้ WebSocket / push) — เหตุผล: simple, ผ่าน firewall ได้ดี, ไม่ต้องการ low-latency

```
Agent (Go)                     Backend (FastAPI)
   │                                  │
   ├──── enroll (1-time token) ──────▶│
   │◀────── agent_token + config ─────┤
   │                                  │
   ├──── heartbeat (every 60s) ──────▶│
   │◀── {config_changed, scan_req} ───┤
   │                                  │
   ├── if scan_required or schedule ──┤
   │   then scan + POST /scans ──────▶│
   │◀────────── 202 Accepted ─────────┤
   │                                  │
   ├── if agent_update_available ────▶│
   │   then GET /binary/latest ──────▶│
   │◀────── download new binary ──────┤
   │   restart self                   │
```

---

## 1. Enrollment

### One-time enrollment token
- Admin สร้างใน dashboard → ได้ token string (32 bytes base64)
- Token valid 24 ชม., ใช้ได้ครั้งเดียว
- Bind กับ tag/group ที่ admin กำหนด (optional)

### Flow
```
Agent: POST /api/v1/agents/enroll
       Headers: X-Enrollment-Token: <one-time>
       Body: { hostname, os, os_version, arch, agent_version }

Backend:
  1. Validate enrollment token (exists, not used, not expired)
  2. Create `machine` row → generate UUID
  3. Generate agent_token (32 bytes random, store bcrypt hash)
  4. Mark enrollment token as used
  5. Return { machine_uuid, agent_token, config }

Agent:
  1. Save machine_uuid + agent_token ใน secure storage:
     - Windows: %ProgramData%\SoftSentry\agent.token (ACL: SYSTEM + Admins only)
     - macOS: /Library/Application Support/SoftSentry/agent.token (root:wheel 600)
  2. Apply config (scan interval, etc.)
  3. Start service loop
```

### Re-enrollment
หาก token หาย หรือ admin revoke:
- Agent detects 401 → delete local token → ขอ enrollment token ใหม่จาก admin → enroll ใหม่
- Old `machine` row จะถูก mark deleted (admin manual)

---

## 2. Heartbeat

**ทุก 60 วินาที** (configurable 30s-300s)

```
Agent: POST /api/v1/agents/heartbeat
       Headers: Authorization: Bearer <agent_token>
       Body: { agent_version, uptime_seconds }

Backend:
  1. Validate agent_token (bcrypt compare against machines.agent_token_hash)
  2. Update machines.last_seen_at = NOW()
  3. Insert heartbeats row
  4. Read agent_configs for this machine
  5. Read latest agent_version available for this platform
  6. Return:
     {
       config_changed: bool,         // ถ้า true, agent ควร GET /agents/config
       manual_scan_requested: bool,   // ถ้า true, scan ทันที + reset flag
       agent_update_available: {version, url} | null
     }
```

### Stale detection
- Status `online` = last_seen_at < 5 min ago
- Status `stale` = 5 min ≤ last_seen_at < 1 hr
- Status `offline` = last_seen_at >= 1 hr
- (computed in query, ไม่ต้อง store)

---

## 3. Scan

### Trigger
1. **Schedule** — agent local cron (default ทุก 6 ชม.)
2. **Manual** — heartbeat response `manual_scan_requested=true` → scan ทันที → reset flag (server side ผ่าน POST /scans success)
3. **Enroll** — scan ครั้งแรกหลัง enrollment

### Scan procedure (Windows)
```go
// pseudocode
sw := []Software{}

// 1. Read uninstall registry
for _, hive := range []string{
    `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
    `HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`,
    `HKCU\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
} {
    for _, key := range readSubkeys(hive) {
        sw = append(sw, parseUninstallEntry(key))
    }
}

// 2. Verify signature for each software's executable (if install_path exists)
for i := range sw {
    if sw[i].InstallPath != "" {
        sw[i].Signature = verifyAuthenticode(sw[i].InstallPath)
    }
}

// 3. POST to backend
POST /api/v1/agents/scans
```

### Scan procedure (macOS)
```go
sw := []Software{}

// 1. Walk /Applications and /System/Applications
for _, dir := range []string{"/Applications", "/System/Applications"} {
    for _, app := range walk(dir, "*.app") {
        info := readInfoPlist(app + "/Contents/Info.plist")
        sw = append(sw, Software{
            Name: info.CFBundleName,
            Version: info.CFBundleShortVersionString,
            Publisher: info.NSHumanReadableCopyright, // best effort
            InstallPath: app,
            Source: "plist",
        })
    }
}

// 2. Read /var/db/receipts/*.plist for pkg installations
// 3. codesign --verify --verbose for each
for i := range sw {
    sw[i].Signature = runCodesign(sw[i].InstallPath)
}

// 4. POST to backend
```

### Signature verification — Windows (Authenticode)
- ใช้ `github.com/saferwall/pe` parse PE file
- Extract embedded signature
- Validate chain ผ่าน `WinVerifyTrust` API ผ่าน `golang.org/x/sys/windows`
- Map result:
  - `TRUST_E_NOSIGNATURE` → `unsigned`
  - `TRUST_E_EXPLICIT_DISTRUST` → `invalid`
  - `CERT_E_EXPIRED` → `expired`
  - `S_OK` → `valid`

### Signature verification — macOS (codesign)
- Shell: `codesign --verify --deep --strict <path>` + `codesign -dvvv <path>` (parse output)
- Map:
  - exit 0 + valid chain → `valid`
  - exit 0 but cert expired → `expired`
  - exit ≠ 0 + "code object is not signed" → `unsigned`
  - else → `invalid`

### Batching
- 1 scan = 1 POST. Body อาจใหญ่ (500 software × 2KB = 1MB) — เปิด gzip request
- ถ้า POST ใหญ่กว่า 10MB → split เป็นหลาย POST (agent ส่ง `scan_session_id` + `chunk_index`/`total_chunks` ใน header) — TBD ใน v1 ค่อยพิจารณา

### Retry
- POST /scans fail → exponential backoff (1s, 2s, 4s, 8s... max 5 min)
- เก็บ result ใน local file-based queue (`queue/` dir, ดู Module 1 §1.7) → ส่งเมื่อ network กลับมา
- queue max 10 scan results (FIFO drop)

---

## 4. Config Pull

```
Agent: GET /api/v1/agents/config
Backend: return agent_configs row for this machine
Agent: apply (e.g. update cron interval)
```

Agent pull config เมื่อ heartbeat response `config_changed=true`

---

## 5. Auto-update

```
heartbeat response: agent_update_available = { version: "1.0.1", url: "/binary/..." }

Agent:
  1. Download → temp file
  2. Verify SHA256 (from response)
  3. Verify signature ของ binary เอง (เซ็นโดย organization)
  4. Replace self:
     - Windows: rename .exe → .exe.old, write new, restart service
     - macOS: similar — LaunchDaemon will restart
```

Auto-update ปิดได้ผ่าน `agent_configs.auto_update_enabled = false`

---

## Wire Format

- **Content-Type**: `application/json; charset=utf-8`
- **Compression**: `Content-Encoding: gzip` สำหรับ request body > 10KB
- **Auth**: `Authorization: Bearer <agent_token>` หรือ `X-Enrollment-Token: ...`
- **User-Agent**: `SoftSentry-Agent/{version} ({os}; {arch})`

## Timeouts

| Operation | Timeout |
|-----------|---------|
| Heartbeat | 10s (connect 3s, read 7s) |
| Scan POST | 60s (large body) |
| Binary download | 5 min |

## Clock drift

ห้าม agent assume clock match server. ทุก timestamp ใน agent ใช้ local clock + ระบุ TZ. Server convert เป็น UTC. **ไม่** validate ว่า `started_at < completed_at` ห่างเกินไป (clock drift ปกติ)

## Idempotency

POST /scans → response มี `scan_uuid`. ถ้า agent retry, ใช้ same scan_uuid ใน header `Idempotency-Key`. Backend de-dup ภายใน 24 ชม.

## Security

- ดูรายละเอียดที่ [`06-security.md`](06-security.md)
- TL;DR: TLS เสมอ (prod), agent token = bearer (rotated เมื่อ revoke), enrollment token 1-time
