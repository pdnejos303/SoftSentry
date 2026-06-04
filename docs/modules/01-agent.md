# Module 1 — Agent

## Purpose

Single-binary endpoint agent ที่:
- สแกน installed software + digital signature
- รายงานผลกลับ backend ตามตารางเวลา + on-demand
- รัน background เป็น Windows Service / macOS LaunchDaemon
- Auto-update binary

## Phase
Phase 2 — implement หลัง Phase 1 (auth + skeleton)

## Out of scope (เฉพาะ v1)
- ❌ Linux
- ❌ ARM Windows (เฉพาะ amd64)
- ❌ Process/file monitoring (เฉพาะ scan inventory)
- ❌ Remote command execution (agent ห้ามรัน command จาก server)

---

## Features & Acceptance Criteria

### 1.1 Auto-scan ตาม schedule
**ตัวกำหนด:** `agent_configs.scan_interval_hours` (default 6, range 1-24)

**Acceptance:**
- ติดตั้ง agent → ครั้งแรก scan ทันทีหลัง enroll สำเร็จ
- ครั้งถัดไปทุก `scan_interval_hours`
- ถ้า service ถูกหยุด/restart → คำนวณเวลา next scan จาก last_scan_at, ไม่ scan ซ้ำเร็วเกิน

### 1.2 Manual scan trigger
**Flow:**
1. Admin ใน dashboard กด "Trigger scan" → `POST /api/v1/machines/{uuid}/trigger-scan`
2. Backend set `agent_configs.manual_scan_requested = true`
3. Agent heartbeat ถัดไป (≤ 60s) เห็น flag → scan ทันที
4. Agent POST scan result → backend reset flag

**Acceptance:**
- กดใน dashboard → scan เริ่มภายใน 60-90s
- ถ้า scan สำเร็จ → flag reset, dashboard show "last scan: just now"
- ถ้า scan fail (network etc) → retry ตาม queue policy, ไม่ reset flag

### 1.3 Scan installed software
**Data per software:**
- name, version (mandatory)
- publisher (optional, จาก registry/plist)
- install_date (optional)
- install_path (optional)
- install_size_kb (optional)
- arch (`x86`/`x64`/`arm64`)
- source (`registry`/`appstore`/`plist`)

**Windows source:**
- `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall` (64-bit)
- `HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall` (32-bit)
- `HKCU\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall` (per-user)
- Skip entries ที่ไม่มี `DisplayName` หรือ `SystemComponent=1`

**macOS source:**
- Walk `/Applications/*.app` + `/System/Applications/*.app`
- อ่าน `Contents/Info.plist` → `CFBundleName`, `CFBundleShortVersionString`, `NSHumanReadableCopyright` (best-effort publisher)
- เพิ่ม `/var/db/receipts/*.plist` สำหรับ pkg-installed software ที่ไม่อยู่ใน /Applications

**Acceptance:**
- Scan เครื่อง Windows มี 100+ software → จับครบทุกอันที่มีใน "Programs and Features"
- Scan เครื่อง macOS → จับครบทุก app ใน /Applications
- ไม่ duplicate (สังเกตว่า WOW6432Node อาจซ้ำกับ Uninstall) — dedupe by `(name, version, install_path)`

### 1.4 Digital signature verification
ดู [Module 3](03-signature-verification.md) สำหรับ display side

**Agent ฝั่งทำ:**
- Windows: เรียก `WinVerifyTrust` ผ่าน Win32 API → map result
- macOS: shell out `codesign --verify --strict <path>` + parse `codesign -dvvv` output

**Per software:**
- Verify ที่ executable หลัก (จาก `install_path\<exe>` หรือ `app/Contents/MacOS/<binary>`)
- ดึง: status, signer CN, issuer CN, cert thumbprint (SHA-256), cert valid from/to, signature algorithm, full chain

**Performance:**
- Verify cache (per session): hash file → ถ้า hash เคย verify แล้วใน scan รอบนี้ → ใช้ cached result
- Parallel verify ไม่เกิน `runtime.NumCPU()` goroutines

**Acceptance:**
- เครื่องที่มี 100 software → scan + verify เสร็จใน ≤ 60 วินาที
- จำแนกถูก: signed valid, signed expired, signed invalid, unsigned (ตรงกับที่ `signtool verify` รายงาน)

### 1.5 Heartbeat
- ทุก 60s (configurable 30-300s ผ่าน `agent_configs`)
- Payload: agent_version, uptime_seconds
- Response: config_changed, manual_scan_requested, agent_update_available

**Acceptance:**
- Network ขาด → agent retry ทุก 60s, ไม่ spam
- กลับมา online → next heartbeat สำเร็จ, status เปลี่ยน online ใน dashboard ภายใน 90s

### 1.6 Auto-update agent
- Binary version hosted ที่ `/api/v1/agents/binary/{version}/{os}/{arch}`
- Heartbeat response มี `agent_update_available = {version, download_url, sha256}` ถ้ามีใหม่กว่า + `auto_update_enabled = true`
- Agent download → verify SHA256 + Authenticode/codesign → replace self → restart service

**Acceptance:**
- Admin upload binary version ใหม่ผ่าน admin tool (TBD CLI) → agents update เองภายใน 24 ชม.
- ถ้า SHA256 mismatch → ทิ้ง download, log error, ยังใช้ version เดิม
- ถ้า restart fail → roll back to .old binary

### 1.7 Local retry queue
- ถ้า POST scan fail → store ลง **durable on-disk queue** ที่ `%ProgramData%\SoftSentry\queue\` (Windows) หรือ `/Library/Application Support/SoftSentry/queue/` (macOS)
- Retry ด้วย exponential backoff (1s, 2s, 4s, ... cap 5 min)
- Max queue 10 scans (FIFO drop เก่าสุดถ้าเกิน)

> **Storage backend — file-based, ไม่ใช่ SQLite (decision 2026-05-30):**
> ดราฟต์แรกระบุ SQLite (`queue.db`) แต่ขัดกับ **กฎ non-negotiable #3 (single binary + cross-compile Win/Mac)** — SQLite driver ที่ใช้ CGO (`mattn/go-sqlite3`) cross-compile ไม่ได้, ส่วน pure-Go (`modernc.org/sqlite`) เพิ่ม binary ~4MB ซึ่ง overkill สำหรับ queue แค่ 10 รายการ.
> **Implementation:** 1 scan = 1 ไฟล์ JSON ใน dir ข้างบน, FIFO เรียงตาม unix-nano prefix ในชื่อไฟล์, crash-safe ด้วย temp-file + `os.Rename` (atomic ทั้ง Windows/POSIX → ได้ transactional guarantee ตาม edge case 5 โดยไม่มี dependency). ดู `agent/internal/queue/`.

**Acceptance:**
- ปิด network → run scan manual → result queue
- เปิด network → ภายใน 5 min ส่งสำเร็จ
- Queue 10 results แล้วเพิ่ม → drop oldest

### 1.8 Service / Daemon installation
**Windows:**
- รัน `softsentry-agent install` (admin) → register SCM service `SoftSentryAgent`
- Auto-start, restart on failure (3 retries)
- Run as `LocalSystem`

**macOS:**
- รัน `sudo softsentry-agent install` → write plist `/Library/LaunchDaemons/com.softsentry.agent.plist`
- `RunAtLoad`, `KeepAlive: true`
- Run as `root`

**Acceptance:**
- หลัง install → reboot เครื่อง → agent กลับมาเอง
- `softsentry-agent uninstall` ลบ service + binary + config (ยกเว้น `queue/` dir ที่มี scan ค้างส่ง — ถามก่อน)

### 1.9 CLI subcommands
```
softsentry-agent --version
softsentry-agent install --enrollment-token <token> [--server <url>]
softsentry-agent uninstall
softsentry-agent enroll --token <token>
softsentry-agent run                          # foreground (debug)
softsentry-agent scan [--output <stdout|json-file>]   # one-shot scan, ไม่ส่ง server
softsentry-agent status
softsentry-agent logs [--tail N]
```

---

## File layout (agent/)

```
agent/
├── cmd/softsentry-agent/main.go
├── internal/
│   ├── config/                  # viper, load from file + env + flag
│   ├── scanner/
│   │   ├── scanner.go           # interface: type Scanner interface { Scan(ctx) ([]Software, error) }
│   │   ├── software.go          # struct Software, SoftwareList
│   │   ├── windows_registry.go  # //go:build windows
│   │   ├── windows_signature.go # WinVerifyTrust wrapper
│   │   ├── macos_apps.go        # //go:build darwin
│   │   └── macos_codesign.go
│   ├── transport/
│   │   ├── client.go            # http client + retry
│   │   ├── enroll.go
│   │   ├── heartbeat.go
│   │   └── scan_post.go
│   ├── queue/
│   │   └── queue.go             # local retry queue (file-based, ดู 1.7)
│   ├── service/
│   │   ├── windows_service.go   # //go:build windows
│   │   └── macos_daemon.go      # //go:build darwin
│   └── updater/
│       └── self_update.go
└── go.mod
```

---

## Security boundaries (agent-specific)

- Agent token stored at: `%ProgramData%\SoftSentry\agent.token` (Windows, ACL: SYSTEM + Admins) / `/Library/Application Support/SoftSentry/agent.token` (macOS, mode 600 root:wheel)
- ไม่อ่านไฟล์ผู้ใช้ (Documents, browser data)
- ห้าม load plugin/extension dynamic
- TLS validate cert chain เสมอ — ถ้าใช้ corporate CA → install ใน OS trust store

---

## Telemetry (Phase 5)

Metrics ที่ส่งกลับ backend (ผ่าน scan metadata, ไม่แยก endpoint):
- scan_duration_ms
- software_count
- signature_verify_errors
- queue_depth
- agent_uptime_seconds

---

## Edge cases ที่ต้อง handle

1. **ชื่อ software มี non-ASCII (Thai, CJK)**: encode UTF-8 ทั้งเส้นทาง — Registry บน Windows คืน UTF-16, แปลงก่อนส่ง
2. **install_path มี space หรือ Unicode**: quote ตอนเรียก codesign, ใช้ syscall.UTF16PtrFromString บน Windows
3. **Same software 2 versions ติดตั้งพร้อมกัน** (e.g. Java 8 + Java 17): 2 records, dedupe key `(name, version)`
4. **Software ใน user-specific HKCU**: ส่งขึ้นด้วย, ไม่ต้องแยก. แต่ flag `source=registry_hkcu` (TBD ใน v2)
5. **Agent ถูกฆ่ากลางคัน scan**: file queue ปลอดภัย — เขียนด้วย temp-file + atomic rename, ไฟล์ที่เขียนค้างถูก skip. Restart → resume queue
6. **Clock drift > 1 ชม.** กับ server: scan ยังส่งได้, server convert UTC แล้วใช้ received_at เป็น authoritative
7. **Disk full**: agent log error, drop queue oldest, retry ภายหลัง — ไม่ crash

---

## Test plan

### Unit
- Parse uninstall entry (เคสครบ + missing fields)
- Dedupe logic
- Version comparison
- Signature result mapping (mock WinVerifyTrust return codes)
- Queue file operations (enqueue/due/remove/reschedule/cap/backoff)

### Integration (ต้องรันบน OS จริง)
- Windows runner: scan + verify Chrome / Notepad++ → expect signature valid
- macOS runner: scan + verify Safari → expect signature valid
- Verify cross-compile: build for windows จาก linux runner สำเร็จ

### Manual QA checklist
- ติดตั้งบน Win10/Win11/macOS 13+ → service start
- Reboot → service กลับมา
- Disconnect network → scan queue → reconnect → ส่งสำเร็จ
- Trigger scan จาก dashboard → ทำงานใน ≤ 90s
- Uninstall → ลบทุกอย่าง

---

## API touchpoints
ดู [API contracts §Agent endpoints](../04-api-contracts.md)
- `POST /agents/enroll`
- `POST /agents/heartbeat`
- `GET /agents/config`
- `POST /agents/scans`
- `GET /agents/binary/latest`
