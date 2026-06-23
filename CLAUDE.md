# SoftSentry — Claude Code Guide

> ไฟล์นี้ Claude Code โหลดอัตโนมัติทุก session — เก็บเฉพาะ context สำคัญที่ต้องรู้ก่อนแตะโค้ด

---

## 🧭 ทิศทางปัจจุบัน — อ่านก่อนวางแผน/เขียนงานใหม่ (สำคัญสุด)

**โปรเจคเปลี่ยนทิศ: SoftSentry = Endpoint Security Posture Monitoring** (ไม่ใช่ "Software Asset Management" แล้ว)

➡️ **ไฟล์หลักอันเดียวที่ต้องอ่านก่อน:** [`docs/security-posture/00-START-HERE.md`](docs/security-posture/00-START-HERE.md)
จากนั้น `docs/security-posture/roadmap.md` (แผน Phase 6–9)

**กฎ:**
- 🟢 ทิศทาง/แผนใหม่ → ยึด `docs/security-posture/` เท่านั้น
- 🔴 แผนเก่าทั้งหมดย้ายไป `docs/OLD-do-not-use/` แล้ว (00–12, `modules/`, `LEARN-*`) + `ROADMAP.md` (root) = **แผนเดิม SAM** — ใช้เข้าใจโค้ดเดิมได้ **แต่ห้ามเอาไปอิงเป็นทิศทางใหม่เด็ดขาด**
- ➕ **Additive only** — ห้ามลบโค้ด/module เดิม (ลายเซ็น/license/CVE ยังอยู่ครบ แค่ลดบทบาท)

---

## Project ที่กำลังทำ

**SoftSentry** — Endpoint Security Posture Monitoring สำหรับองค์กร

agent บนเครื่อง endpoint (Windows/Mac) สแกน software inventory + **provenance (มาจากไหน/ใครลง)** + signature + CVE + device posture → ส่ง backend → ตัดสิน **"ปลอดภัยไหม"** + แจ้งเตือน → IT admin ดู dashboard

**สถานะ:** Module 1–9 (SAM เดิม) code-complete แล้ว · ตอนนี้กำลังต่อ Phase 6–9 ทิศ security posture — ดู `docs/security-posture/`

---

## Tech Stack

| Layer | Stack | หมายเหตุ |
|-------|-------|----------|
| Agent | **Go 1.22+** | Single binary, cross-compile Win/Mac, ใช้ `golang.org/x/sys/windows/registry`, Authenticode verify |
| Backend | **Python 3.12 + FastAPI** | Async, auto OpenAPI, SQLAlchemy 2.x + Alembic |
| Frontend | **Next.js 14 (App Router) + TypeScript + shadcn/ui + Tailwind** | React Query, Recharts |
| DB | **PostgreSQL 16** | Run ใน Docker |
| Infra | **Docker Compose** dev + prod | Nginx reverse proxy |
| Observability | **Prometheus + Grafana** | metrics จากทั้ง agent + backend |
| i18n | **next-intl** (frontend) | รองรับ ไทย/EN |

(stack ยังเหมือนเดิม — เหตุผลการเลือกฉบับเดิมอยู่ที่ `docs/OLD-do-not-use/01-tech-stack.md`; i18n ตอนนี้รองรับ ไทย/EN/JA)

---

## Directory Layout (ที่จะสร้าง)

```
SoftSentry/
├── agent/                 # Go source — single binary
│   ├── cmd/
│   ├── internal/
│   │   ├── scanner/       # Software inventory scan
│   │   ├── signature/     # Authenticode/Codesign verify
│   │   ├── transport/     # HTTP client → backend
│   │   └── service/       # Windows Service / launchd wrapper
│   └── go.mod
├── backend/               # FastAPI
│   ├── app/
│   │   ├── api/v1/        # Routers แยกตาม module
│   │   ├── core/          # config, security, db
│   │   ├── models/        # SQLAlchemy models
│   │   ├── schemas/       # Pydantic schemas
│   │   ├── services/      # Business logic
│   │   └── workers/       # Background tasks (CVE sync, etc)
│   ├── alembic/
│   ├── tests/
│   └── pyproject.toml
├── dashboard/             # Next.js
│   ├── app/
│   │   ├── (auth)/
│   │   ├── (dashboard)/
│   │   └── api/
│   ├── components/
│   ├── lib/
│   └── package.json
├── docs/                  # Spec ทั้งหมด (อ่านก่อนเขียน code)
└── docker-compose.yml
```

---

## หลักการที่ต้องยึด (Non-negotiable)

1. **อ่าน spec ก่อนเขียน code** — ทิศทางใหม่อยู่ใน `docs/security-posture/` · spec ของ module เดิม (1–9) อยู่ใน `docs/OLD-do-not-use/modules/`
2. **TDD เมื่อเป็นไปได้** — เขียน test ก่อน implementation โดยเฉพาะ business logic (vulnerability matching, license calculation)
3. **Agent ต้องเป็น single binary** — ห้าม depend on runtime/interpreter ที่ผู้ใช้ต้องลงเพิ่ม
4. **Multi-platform from day 1** — code agent ต้องรองรับทั้ง Windows และ macOS ตั้งแต่แรก
5. **Security by default** — ทุก API ผ่าน auth, agent enrollment ใช้ token, secrets ไม่อยู่ใน code
6. **i18n-aware** — ทุก user-facing string ใน dashboard ผ่าน next-intl (ห้าม hardcode ภาษา)

---

## ลำดับการ implement

ทิศทางปัจจุบัน: ดู [`docs/security-posture/roadmap.md`](docs/security-posture/roadmap.md) (Phase 6–9) · roadmap เก่า (Phase 1–5, เสร็จแล้ว) อยู่ที่ `ROADMAP.md` (root, มีป้ายเก่า)

**ห้าม implement หลาย module พร้อมกัน** จนกว่า phase ก่อนหน้าเสร็จและ test ผ่าน

---

## เอกสารที่ต้องอ่าน

| งาน | อ่าน |
|-----|------|
| ทิศทาง/แผนใหม่ (ทุกงาน) | `docs/security-posture/00-START-HERE.md` → `roadmap.md` → spec ของ phase นั้น |
| เข้าใจโค้ดเดิม (Module 1–9) | `docs/OLD-do-not-use/modules/0X-*.md` |
| convention / testing เดิม | `docs/OLD-do-not-use/09-coding-conventions.md`, `10-testing-strategy.md` |
| วิธีรันระบบ | `docs/RUN.md`, `docs/dev-setup.md` |

---

## Convention สรุปสั้นๆ (รายละเอียดเดิมใน `docs/OLD-do-not-use/09-coding-conventions.md`)

- **Go**: `gofmt`, `golangci-lint`, errors wrap ด้วย `fmt.Errorf("...: %w", err)`, ไม่ใช้ panic ใน production path
- **Python**: `ruff` + `black`, type hints บังคับ, `async def` สำหรับ I/O ทุกตัว, Pydantic v2 models
- **TypeScript**: `eslint` + `prettier`, `strict: true`, ห้าม `any`, React Server Components by default

---

## Build/Test Commands

```bash
# Agent
cd agent && go build ./cmd/softsentry-agent
cd agent && go test ./...

# Backend
cd backend && uv run uvicorn app.main:app --reload
cd backend && uv run pytest

# Dashboard
cd dashboard && pnpm dev
cd dashboard && pnpm test

# All-in-one dev environment
docker compose up -d
```

---

## เมื่อไม่แน่ใจ

- spec ไม่ชัด → **ถามผู้ใช้** ก่อนเดา. อย่า invent feature
- conflict ระหว่าง modules → อ่าน `docs/OLD-do-not-use/02-architecture.md` ตัดสิน
- security decision → default ไปทางปลอดภัยกว่า, ถามถ้าจะ trade-off

---

## Design Context

> Source of truth สำหรับงาน frontend/ดีไซน์ทั้งหมด — รายละเอียดเต็มอยู่ที่ [`.impeccable.md`](.impeccable.md) (root). อ่านส่วนนี้ก่อนแตะ UI ทุกครั้ง

**Users:** Corporate IT admins / SOC staff เฝ้า endpoint security posture ทั้ง fleet (Win/macOS) — นั่งจ้องนาน ๆ บนจอใหญ่ (บางทีเป็นจอ NOC) งานข้อมูลแน่น: machines, software, signatures, CVE, licenses, policy, alerts, audit. การตัดสินใจมีผล ("เครื่องนี้ปลอดภัยไหม") → UI ห้ามสร้างดราม่า/ความกำกวมเอง

**Brand personality:** Precise · Dependable · Quiet — เครื่องมือ security ที่ไว้ใจได้ตอนตีสอง พูดด้วยข้อเท็จจริง ไม่ตะโกน ความมั่นใจมาจากความชัดเจน+ความนิ่ง ไม่ใช่การตกแต่ง เป้าหมายอารมณ์ = **calm authority**

**Aesthetic — "Color is signal, not decoration":**
- พื้น (canvas) = neutral เย็น ๆ tint เข้าหา iris hue (ไม่ใช่เทาด้าน ๆ, ไม่ใช้ดำ/ขาวล้วน)
- **สี saturated สงวนไว้ให้ risk semantics เป็นหลัก:** 🟢 safe/verified=green · 🟡 warning=amber · 🔴 critical=red · ⚪ unknown=neutral grey
- **Signature accent = cool desaturated iris-indigo** (ตั้งใจให้ต่างจาก shadcn 221° blue และไม่ชนสีความเสี่ยง) ใช้แบบ flat + ประหยัด: active nav, focus ring, primary action, link, selection — **ห้าม glow/gradient**
- **Theme:** light + dark first-class ทั้งคู่ · dark = blue-charcoal เข้ม (ไม่ใช่ดำล้วน + neon) · light = cool off-white (ไม่ใช่ #fff)
- **Anti-refs (ห้ามออกมาเป็น):** stock shadcn blue-on-slate · "AI slop" (cyan-on-dark, purple→blue gradient, neon glow, glassmorphism, gradient text บนตัวเลข, card grid ไอคอนซ้ำ ๆ, ยัดทุกอย่างใน card) · consumer/playful SaaS · bounce/elastic motion · mono ใช้มั่ว (mono เฉพาะ machine data จริง: hash/ID/version/thumbprint)

**5 Principles:** ① Color = risk signal (canvas นิ่ง สงวน saturation ให้ความหมาย) · ② Density with rhythm (แน่นได้แต่มี spacing rhythm + hierarchy) · ③ อ่านออกจากไกล + แม่นยำตอนซูม · ④ Restraint over flourish (motion = state change เท่านั้น, ease-out, ห้าม bounce) · ⑤ Tri-lingual (TH/EN/JA) + dual-theme ตั้งแต่ต้น
