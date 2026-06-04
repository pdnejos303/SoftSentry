# 📚 LEARN — เข้าใจ SoftSentry ทั้งระบบ (สำหรับคนที่เคยทำแค่ Next CRUD)

> เอกสารนี้ไม่ใช่ spec — มันคือ "ครู" ที่พาคุณจากสิ่งที่คุณรู้ (Next.js CRUD ง่ายๆ) ไปสู่ stack เต็มของ SoftSentry
> อ่านจบแล้วคุณจะตอบได้ว่า **แต่ละชิ้นคืออะไร / ทำงานยังไง / ทำงานด้วยกันยังไง / ใช้อะไรทำ**
> อ่านคู่กับ `docs/01-tech-stack.md` (เหตุผลการเลือก) และ `docs/02-architecture.md` (ภาพรวม)

---

## สารบัญ

1. [เริ่มจากสิ่งที่คุณรู้: Next CRUD ต่างกับ SoftSentry ยังไง](#1)
2. [ตัวละครทั้ง 7 ตัวในระบบ — แต่ละตัวคือใคร](#2)
3. [เจาะรายตัว #1 — Agent (Go)](#3)
4. [เจาะรายตัว #2 — Backend (Python + FastAPI)](#4)
5. [เจาะรายตัว #3 — Worker + Redis (งานเบื้องหลัง)](#5)
6. [เจาะรายตัว #4 — Dashboard (Next.js 14)](#6)
7. [เจาะรายตัว #5 — PostgreSQL](#7)
8. [เจาะรายตัว #6 — Docker / Nginx (กาวที่ยึดทุกอย่าง)](#8)
9. [ทุกตัวทำงานด้วยกันยังไง — เดินตาม data flow จริง](#9)
10. [Glossary — คำศัพท์ใหม่ที่จะเจอ](#10)
11. [แผนที่: ถ้าจะแตะ X ต้องรู้อะไรบ้าง](#11)

---

<a name="1"></a>
## 1. เริ่มจากสิ่งที่คุณรู้: Next CRUD ต่างกับ SoftSentry ยังไง

ตอนทำ Next CRUD ปกติ โลกของคุณมี **3 ชิ้น**:

```
[Browser/React] ──fetch──▶ [Next.js API route] ──query──▶ [Database]
```

- ผู้ใช้คน = คนกดปุ่มในเบราว์เซอร์
- ทุกอย่างเริ่มจาก "ผู้ใช้กดอะไรสักอย่าง" แล้วระบบตอบกลับทันที (request → response จบ)
- ภาษาเดียว (TypeScript/JavaScript) ตั้งแต่หน้าจอยันฐานข้อมูล

**SoftSentry ต่างออกไป 4 เรื่องใหญ่** ที่ทำให้มันซับซ้อนกว่า CRUD:


| เรื่อง | Next CRUD | SoftSentry | ทำไมต้องเปลี่ยน |
|--------|-----------|-----------|----------------|
| **ใครเป็นคนเริ่มงาน** | คนกดปุ่ม | **โปรแกรม (agent) บนเครื่องพนักงาน 1,000 เครื่อง** ส่งข้อมูลเข้ามาเองทุก 6 ชม. | เราต้องรู้ว่าเครื่องพนักงานมี software อะไร โดยไม่ต้องให้พนักงานทำอะไรเลย |
| **งานเสร็จทันทีไหม** | เกือบทุกงานตอบกลับใน <1 วินาที | บางงานใช้เวลานาน (sync ฐานข้อมูล CVE 250,000 รายการ, gen PDF) | งานนานต้อง "แยกไปทำเบื้องหลัง" ไม่งั้นผู้ใช้รอจนหน้าค้าง |
| **กี่ภาษา** | 1 ภาษา | **3 ภาษา**: Go (agent), Python (backend), TypeScript (dashboard) | แต่ละงานเหมาะกับเครื่องมือคนละแบบ (อธิบายข้อ 2) |
| **รันที่ไหน** | 1 process | **6 process แยกกัน** คุยกันผ่าน network | แยกหน้าที่ → ตัวไหนพังไม่ลากตัวอื่นล้ม, scale แยกได้ |

> 💡 **กุญแจสำคัญ:** ใน Next CRUD ทุกอย่าง "เริ่มจากคน". ใน SoftSentry มี 2 แหล่งที่เริ่มงาน — **คน** (admin เปิด dashboard) และ **เครื่องจักร** (agent ส่งข้อมูลเข้ามาเอง + ตัวจับเวลายิงงานเบื้องหลัง). พอเข้าใจจุดนี้ ทุกอย่างที่เหลือจะเริ่ม make sense

---

<a name="2"></a>
## 2. ตัวละครทั้ง 7 ตัวในระบบ — แต่ละตัวคือใคร

ลองคิดว่า SoftSentry เป็น "บริษัทรักษาความปลอดภัย" บริษัทหนึ่ง มีพนักงาน 7 ตำแหน่ง:

```
                            🌐 INTERNET
                                 │
   พนักงานในองค์กร 1,000 คน      │  IT Admin เปิดเบราว์เซอร์
   แต่ละเครื่องมี 👇            │         │
┌─────────────────────┐         │         ▼
│  1. AGENT (Go)      │         │   ┌──────────────────┐
│  "สายลับประจำเครื่อง"│─HTTPS──▶│   │  6. NGINX        │  ◀── ยามหน้าประตู
└─────────────────────┘         │   │  (reverse proxy) │
                                │   └────────┬─────────┘
                                │       ┌────┴────┐
                                ▼       ▼         ▼
                        ┌──────────────┐   ┌──────────────┐
                        │ 2. BACKEND   │   │ 4. DASHBOARD │
                        │   (FastAPI)  │   │  (Next.js)   │
                        │ "พนักงาน     │   │ "หน้าร้าน +  │
                        │  รับเรื่อง"  │   │  รายงาน"     │
                        └──────┬───────┘   └──────────────┘
                               │
              ┌────────────────┼─────────────────┐
              ▼                ▼                  ▼
      ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
      │ 3. WORKER    │  │ 5. POSTGRES  │  │ (Redis)      │
      │   (arq)      │  │   ฐานข้อมูล  │  │ กล่องคิวงาน  │
      │ "ฝ่ายหลังบ้าน│  │ "ตู้เอกสาร   │  │  + ตู้เย็น    │
      │  งานหนัก"    │  │  ตัวจริง"    │  │  (cache)     │
      └──────────────┘  └──────────────┘  └──────────────┘
```

ไล่ทีละตัว — **หน้าที่ / ใช้อะไรเขียน / ทำไมต้องมี**:

| # | ตัวละคร | เปรียบเทียบ | หน้าที่หลัก | เขียนด้วย |
|---|---------|-----------|-----------|-----------|
| 1 | **Agent** | สายลับประจำเครื่องพนักงาน | สแกนว่าเครื่องนี้มี software อะไร, ตรวจลายเซ็นดิจิทัล, ส่งกลับ backend ทุก 6 ชม. | **Go** |
| 2 | **Backend API** | พนักงานเคาน์เตอร์รับเรื่อง | รับข้อมูลจาก agent + คำขอจาก dashboard, ตรวจสิทธิ์, อ่าน/เขียนฐานข้อมูล, ตอบกลับทันที | **Python (FastAPI)** |
| 3 | **Worker** | ฝ่ายหลังบ้านงานหนัก | ทำงานที่ใช้เวลานาน: ดึงฐานข้อมูล CVE, จับคู่ช่องโหว่, ทำ PDF, เช็ค license หมดอายุ | **Python (arq)** |
| 4 | **Dashboard** | หน้าร้าน + ห้องรายงาน | UI ที่ IT admin เห็น: รายการเครื่อง, กราฟ, แจ้งเตือน | **TypeScript (Next.js)** |
| 5 | **PostgreSQL** | ตู้เอกสารตัวจริง (source of truth) | เก็บข้อมูลถาวรทุกอย่าง: users, machines, software, CVE, alerts | (SQL) |
| 6 | **Nginx** | ยามหน้าประตู | รับ request จากเน็ตที่เดียว แล้วแจกไป backend หรือ dashboard, จัดการ TLS/HTTPS | (config) |
| 7 | **Redis** | กล่องคิวงาน + ตู้เย็น | คิวงานให้ worker (job queue), cache ข้อมูลชั่วคราว, นับจำนวน (rate limit) | (in-memory) |

> 🤔 **"ทำไมไม่ทำทุกอย่างเป็น Next.js เหมือนที่เคยทำ?"**
> - Agent ต้องเป็นไฟล์เดียวจบ รันบนเครื่องพนักงานที่ไม่มี Node.js → ใช้ **Go** ที่ compile เป็น .exe เดียว
> - Backend ต้องจับคู่ CVE/วิเคราะห์ security → โลก Python มี library ด้านนี้เยอะสุด
> - Dashboard เป็นงานที่ Next.js ถนัดอยู่แล้ว → ใช้ Next.js
>
> **"ใช้เครื่องมือที่เหมาะกับงาน" ไม่ใช่ "ใช้เครื่องมือเดียวกับทุกงาน"** — นี่คือก้าวแรกของการคิดแบบ system ไม่ใช่แค่ app

---

<a name="3"></a>
## 3. เจาะรายตัว #1 — Agent (Go)

### มันคืออะไร
โปรแกรมตัวเล็กๆ (ไฟล์ `.exe` ตัวเดียวบน Windows, ไฟล์ binary ตัวเดียวบน Mac) ที่ติดตั้งไว้บนเครื่องพนักงาน แล้ว **รันอยู่เบื้องหลังตลอดเวลา** เหมือนโปรแกรมแอนตี้ไวรัส คุณไม่เห็นหน้าต่างมันเลย มันเงียบๆ ทำงานของมัน

### มันทำอะไรบ้าง (วน loop ตลอดชีวิต)
```
ทุก 60 วินาที  → ส่ง "heartbeat" บอก server ว่า "ฉันยังอยู่นะ" + ถามว่ามีงานสั่งมาไหม
ทุก 6 ชั่วโมง  → "scan": ไล่ดูว่าเครื่องนี้ลง software อะไรบ้าง + ตรวจลายเซ็น → ส่งขึ้น server
เมื่อ server สั่ง → scan ทันที (ไม่ต้องรอครบ 6 ชม.)
เมื่อมีเวอร์ชันใหม่ → ดาวน์โหลดตัวเองเวอร์ชันใหม่ มาแทนที่ตัวเก่า (auto-update)
```

### ทำไมต้องเป็น Go (ไม่ใช่ Python/Node ที่อาจคุ้นกว่า)?
นี่คือเหตุผลที่สำคัญที่สุดของทั้งโปรเจกต์:

- **Single binary** — สั่ง `go build` ครั้งเดียวได้ไฟล์เดียวจบ เอาไปวางบนเครื่องไหนก็รันได้เลย **ไม่ต้องลง Python, ไม่ต้องลง Node, ไม่ต้องลงอะไรเพิ่ม** ลองคิดว่าถ้าต้องไปลง Python บนเครื่องพนักงาน 1,000 เครื่องก่อน — ฝันร้าย
- **Cross-compile** — นั่งเครื่อง dev เครื่องเดียว สั่ง build ออกมาได้ทั้งเวอร์ชัน Windows และ Mac
- **กินทรัพยากรน้อย** — RAM ≤ 50MB ตอนอยู่เฉยๆ (ข้อกำหนดใน `docs/00-overview.md`) เพราะมันต้องรันค้างตลอดบนเครื่องพนักงานโดยไม่ทำให้เครื่องอืด
- **Goroutine** — Go มีวิธีทำงานหลายอย่างพร้อมกันที่ง่ายมาก เช่น ตรวจลายเซ็น software 200 ตัวพร้อมกัน

### concept ใหม่ใน Go ที่ต้องรู้ (เทียบกับ JS/TS)

| Go | เทียบกับที่คุณรู้ | หมายเหตุ |
|----|------------------|----------|
| `goroutine` | คล้าย `Promise.all` แต่เป็น "thread เบาๆ" จริงๆ | `go doScan()` = "ไปทำอันนี้เป็นเบื้องหลัง" |
| `error` เป็นค่า return | ไม่มี `try/catch` แบบ JS | ทุกฟังก์ชันคืน `(result, error)` ต้องเช็ค error ทุกครั้ง |
| `struct` | เหมือน `interface`/`type` ใน TS | โครงสร้างข้อมูล |
| compile เป็น binary | ไม่มี `node_modules`, ไม่มี runtime | นี่แหละคือพลังของ Go |

### Agent ทำ "ตรวจลายเซ็นดิจิทัล" ยังไง (Authenticode / codesign)
นี่คือหัวใจด้าน security ของ agent — ขออธิบายเพราะเป็นเรื่องใหม่:

ทุกโปรแกรมที่ถูกต้องตามกฎหมาย (Chrome, Office) จะถูก **"เซ็นชื่อดิจิทัล"** โดยบริษัทผู้ผลิต เหมือนตราประทับรับรอง agent จะเช็คว่า:
- มีลายเซ็นไหม? (ไม่มี = `unsigned` = น่าสงสัย อาจเป็นมัลแวร์)
- ลายเซ็นยังไม่หมดอายุ? (หมดอายุ = `expired`)
- ลายเซ็นถูกต้องจริง ไม่โดนปลอม? (ปลอม = `invalid`)
- ทุกอย่างโอเค = `valid`

บน Windows ใช้ Windows API ชื่อ `WinVerifyTrust`, บน Mac ใช้คำสั่ง `codesign` (ดูรายละเอียดใน `docs/05-agent-protocol.md` §3)

### โครงสร้างโค้ด agent
```
agent/
├── cmd/                    # จุดเริ่ม (main) + คำสั่ง CLI (install, uninstall, status)
└── internal/
    ├── scanner/            # ไล่อ่าน Registry (Win) / โฟลเดอร์ Applications (Mac)
    ├── signature/          # ตรวจลายเซ็น Authenticode/codesign
    ├── transport/          # คุยกับ backend ผ่าน HTTP
    └── service/            # ห่อให้รันเป็น Windows Service / Mac LaunchDaemon
```

> 📌 **สถานะปัจจุบัน:** agent เขียนเสร็จแล้ว (Phase 2) — เคยรันจริงบน Windows เจอ 177 apps. ดู `ROADMAP.md`

---

<a name="4"></a>
## 4. เจาะรายตัว #2 — Backend (Python + FastAPI)

### มันคืออะไร
นี่คือ **"สมอง"** ของระบบ และเป็นตัวที่คุ้นที่สุดสำหรับคุณ เพราะมันก็คือ **"API route" แบบที่คุณเคยทำใน Next** นั่นแหละ — แค่เขียนด้วย Python และมีโครงสร้างจริงจังกว่า

ใน Next คุณเคยเขียนแบบนี้:
```typescript
// app/api/machines/route.ts
export async function GET() {
  const machines = await db.machine.findMany();
  return Response.json(machines);
}
```

ใน FastAPI หน้าตาแบบนี้ (concept เดียวกันเป๊ะ):
```python
# app/api/v1/machines.py
@router.get("/machines")
async def list_machines(db: AsyncSession = Depends(get_db)):
    machines = await machine_service.list_all(db)
    return machines   # FastAPI แปลงเป็น JSON ให้อัตโนมัติ
```

### concept ใหม่ที่ต้องเข้าใจ

**(ก) `async def` / `await` — แต่จริงจังกว่า Next**
- คุณเคยเห็น `async/await` ใน JS อยู่แล้ว concept เหมือนกัน: "ระหว่างรอ I/O (รอ DB, รอ network) อย่านั่งเฉยๆ ไปทำงานอื่นก่อน"
- ใน SoftSentry **บังคับว่างาน I/O ทุกตัวต้องเป็น `async`** (อ่านใน `CLAUDE.md`) เพราะ backend 1 ตัวต้องรับ agent 1,000 ตัวที่ยิง heartbeat ทุก 60 วินาที = ~17 request/วินาที ถ้าแต่ละ request นั่งรอ DB แบบ blocking ระบบจะตันทันที

**(ข) Pydantic — "ตัวตรวจของหน้าด่าน"**
- ปัญหาใน CRUD ทั่วไป: ข้อมูลที่ client ส่งมาเชื่อไม่ได้ ต้อง validate เอง (เขียน if เช็คเยอะแยะ)
- Pydantic = คุณ "ประกาศรูปร่างข้อมูล" ไว้ แล้วมันตรวจให้อัตโนมัติ ถ้าผิดรูป → ตอบ error 422 ให้เลย ไม่ต้องเขียน if เอง
```python
class ScanRequest(BaseModel):
    started_at: datetime
    software: list[SoftwareItem]   # ถ้า client ส่ง software ไม่ครบ field → reject อัตโนมัติ
```
- มี 2 ประเภทในโปรเจกต์นี้: **schemas** (Pydantic — รูปร่างข้อมูลเข้า/ออก API) กับ **models** (SQLAlchemy — รูปร่างข้อมูลในตาราง DB) อย่าสับสน 2 อันนี้

**(ค) SQLAlchemy — ORM (เหมือน Prisma ที่คุณอาจเคยใช้)**
- ORM = เขียน Python แทนการเขียน SQL ดิบ `db.query(Machine).filter(...)` แทน `SELECT * FROM machines WHERE ...`
- เทียบ Prisma: `prisma.machine.findMany()` ↔ SQLAlchemy `select(Machine)`
- โปรเจกต์เลือก SQLAlchemy เพราะมันคือ ORM ที่ mature ที่สุดในโลก Python

**(ง) Alembic — "migration" (จุดที่ต่างจาก Prisma)**
- "migration" = ไฟล์ที่บันทึก "การเปลี่ยนแปลงโครงสร้างตาราง" ทีละขั้น เช่น "เพิ่มคอลัมน์ `tags` ในตาราง machines"
- ทำไมต้องมี? เพราะ DB จริงมีข้อมูลอยู่แล้ว คุณแก้ตารางมั่วไม่ได้ ต้องมี "ประวัติการแก้" ที่รันซ้ำได้และ rollback ได้
- ใน Prisma คือ `prisma migrate`. ใน Python คือ **Alembic**
- กฎเหล็กในโปรเจกต์ (`docs/03-data-model.md`): **ห้ามแก้ migration ที่ apply ไปแล้ว — เขียนอันใหม่เสมอ** และต้องเขียน `downgrade()` (วิธี rollback) ทุกครั้ง

**(จ) JWT — การ login (น่าจะเคยเจอบ้าง)**
- ผู้ใช้ login → backend คืน "token" (สตริงยาวๆ เข้ารหัส) → ทุก request ถัดไป client แนบ token นี้ใน header `Authorization: Bearer ...`
- backend แกะ token ดูว่า "นี่ใคร, role อะไร (admin/viewer)" โดยไม่ต้อง query DB ทุกครั้ง
- รหัสผ่านเก็บแบบ **bcrypt hash** ไม่เคยเก็บ plain text

### โครงสร้างโค้ด backend (สำคัญ — จำ pattern นี้ไว้)
```
backend/app/
├── main.py        # จุดเริ่ม FastAPI app
├── core/          # config, security (JWT/bcrypt), db (session)
├── models/        # 🗄️  SQLAlchemy — รูปร่างตารางใน DB
├── schemas/       # 📋  Pydantic — รูปร่าง JSON เข้า/ออก API
├── api/v1/        # 🚪  Routers — 1 ไฟล์ต่อ 1 module (machines.py, alerts.py...)
├── services/      # 🧠  Business logic — ตรรกะจริง (จับคู่ CVE, คำนวณ risk score)
└── workers/       # ⚙️  งานเบื้องหลัง (ดูข้อ 5)
```

> 💡 **Pattern สำคัญที่ต่างจาก Next CRUD:** สังเกตว่า **logic ไม่อยู่ใน route** มันถูกแยกไป `services/`. route ทำหน้าที่แค่ "รับ request → เรียก service → คืน response". ส่วน service คือที่เก็บสมองจริง วิธีนี้ทำให้ test ง่าย (test service โดยไม่ต้องยิง HTTP จริง) และเป็น pattern ที่ scale ได้ดีกว่ายัดทุกอย่างใน route

> 📌 **สถานะ:** Backend Phase 1-3 ส่วนใหญ่เสร็จแล้ว (auth, inventory, signature, unauthorized detection). กำลังจะทำ Module 5 (CVE matching)

---

<a name="5"></a>
## 5. เจาะรายตัว #3 — Worker + Redis (งานเบื้องหลัง)

นี่คือ concept ที่ **ไม่มีใน Next CRUD ทั่วไป** และเป็นสิ่งที่คุณต้องเข้าใจให้ได้

### ปัญหาที่ worker มาแก้
ลองนึกภาพ: agent ส่ง scan เข้ามา (software 500 ตัว) backend ต้องเอา software 500 ตัวนี้ไปจับคู่กับฐานข้อมูล CVE ที่มี 250,000 รายการ — งานนี้อาจใช้เวลา 30 วินาที

ถ้าทำใน request ตรงๆ → agent ต้องนั่งรอ 30 วินาที, connection ค้าง, ถ้ามี 100 เครื่องส่งพร้อมกัน backend ตาย

**ทางแก้: แยกงานหนักออกไปทำเบื้องหลัง**

```
Agent → POST /scans → Backend: "รับไว้แล้วนะ เก็บลง DB เรียบร้อย" → ตอบ 202 ทันที (เร็ว!)
                          │
                          └─ โยน "ใบสั่งงาน" เข้าคิว Redis: "ไปจับคู่ CVE ให้เครื่องนี้ที"
                                            │
                          Worker หยิบใบสั่งงานจากคิว ──┘
                          → ค่อยๆ ทำ 30 วินาที (ไม่มีใครรอ)
                          → เขียนผลลง DB
```

### Redis คืออะไร (มี 3 หน้าที่)
Redis = ฐานข้อมูลที่เก็บใน RAM (เร็วมาก แต่ข้อมูลไม่ถาวรเท่า Postgres) ในโปรเจกต์นี้ใช้ 3 อย่าง:

1. **Job queue (คิวงาน)** — กล่องที่ backend หย่อน "ใบสั่งงาน" ไว้ แล้ว worker มาหยิบไปทำ. library ที่ใช้ชื่อ **`arq`** (มันใช้ Redis เป็นที่เก็บคิว)
2. **Cache (ตู้เย็น)** — เก็บผลลัพธ์ที่ query บ่อยๆ ไว้ชั่วคราว (เช่น รายการเครื่อง) มี TTL (อายุ) สั้นๆ พอหมดอายุก็ query ใหม่ ลดภาระ Postgres
3. **Rate limiting (ตัวนับ)** — นับว่า IP นี้ login กี่ครั้งใน 5 นาที ถ้าเกิน 5 ครั้ง → บล็อก (กัน brute-force)

### Worker ทำงานอะไรบ้าง
worker เป็น process **แยกต่างหาก** จาก backend (รันคนละ container) มันวนหยิบงานจากคิวมาทำ งานในโปรเจกต์นี้:
- **CVE sync** — ทุกวันไปดึงฐานข้อมูลช่องโหว่ล่าสุดจาก NVD/OSV (เว็บของรัฐบาลสหรัฐฯ) มาเก็บไว้ในเครื่องเรา
- **จับคู่ alert** — scan ใหม่เข้ามา → เช็คว่ามี software ใน blacklist ไหม → สร้าง alert
- **เช็ค license หมดอายุ** — ทุกวันดูว่า license ไหนใกล้หมด (30/60/90 วัน) → เตือน
- **ทำ PDF report** — งานหนัก ทำเบื้องหลังแล้วค่อยบอกว่าเสร็จ

> 💡 **กฎที่เขียนไว้ใน `docs/02-architecture.md`:** *"ไม่ execute long-running tasks ใน request — โยนเข้า worker queue"* นี่คือหลักการที่แยก "ระบบจริงจัง" ออกจาก "CRUD ธรรมดา". เมื่อไหร่ที่งานใช้เวลา > 1-2 วินาที หรือทำงานตามเวลา (ทุกวัน/ทุกชั่วโมง) → มันเป็นงานของ worker ไม่ใช่ของ backend

> 📌 **สถานะ:** worker skeleton มีแล้ว (Phase 1) แต่ตัวงานจริง (CVE sync ฯลฯ) จะเริ่มทำใน Module 5 เป็นต้นไป

---

<a name="6"></a>
## 6. เจาะรายตัว #4 — Dashboard (Next.js 14)

ข่าวดี: **นี่คือบ้านของคุณ** คุณทำ Next มาแล้ว แต่ SoftSentry ใช้ Next.js 14 แบบ **App Router** ซึ่งมี concept ใหม่ที่อาจต่างจากที่คุณเคยทำ (ถ้าคุณเคยทำ Pages Router หรือ CRUD ง่ายๆ)

### ความต่างใหญ่ที่สุด: Server Components (RSC)
ใน Next แบบเก่า (และ CRUD ทั่วไป) ทุก component รันบนเบราว์เซอร์ แล้วค่อย `fetch` ข้อมูลจาก API

ใน App Router **component รันบน server เป็นค่าเริ่มต้น** — มัน fetch ข้อมูลตอน render บน server เลย ส่ง HTML ที่มีข้อมูลพร้อมแล้วไปให้เบราว์เซอร์:

```tsx
// app/(dashboard)/machines/page.tsx — นี่คือ Server Component (ไม่มี 'use client')
export default async function MachinesPage() {
  const machines = await fetch('http://backend/api/v1/machines').then(r => r.json());
  return <MachineTable data={machines} />;   // fetch บน server, ส่ง HTML พร้อมข้อมูล
}
```

- **ข้อดี:** หน้าโหลดเร็ว (ผู้ใช้เห็นข้อมูลทันที ไม่เห็นหน้าว่างแล้วค่อยโหลด), bundle เล็กลง, SEO ดี
- **เมื่อไหร่ต้อง `'use client'`?** เฉพาะ component ที่ต้อง interactive — มีปุ่มกด, useState, onClick, animation. กฎในโปรเจกต์: *"Server Components by default, `'use client'` เฉพาะที่ต้อง interactive"*

### เครื่องมือใน dashboard (จากตาราง tech-stack)

| ใช้ทำ | Library | เทียบกับที่คุณอาจรู้ |
|--------|---------|---------------------|
| **UI components** | `shadcn/ui` | ไม่ใช่ library ที่ติดตั้ง แต่ "copy โค้ดมาไว้ในโปรเจกต์" แก้ได้เต็มที่ |
| **Styling** | `tailwindcss` | น่าจะคุ้นแล้ว |
| **ดึงข้อมูลฝั่ง client** | `@tanstack/react-query` | จัดการ fetch + cache + refetch อัตโนมัติ (แทนการเขียน `useEffect` + `fetch` เอง) |
| **กราฟ** | `recharts` | วาด chart |
| **ตาราง** | `@tanstack/react-table` | ตารางที่ sort/filter/paginate ได้ |
| **ฟอร์ม** | `react-hook-form` + `zod` | จัดการฟอร์ม + validate |
| **หลายภาษา** | `next-intl` | ไทย/EN — **ห้าม hardcode ข้อความ** ทุกข้อความต้องผ่านตัวนี้ |

> ⚠️ **กฎเหล็กที่ต่างจาก CRUD ทั่วไป:** *"ห้าม fetch ใน `useEffect`"* — ถ้าจะดึงข้อมูล ให้ใช้ React Query (ฝั่ง client) หรือ fetch ใน Server Component (ฝั่ง server). การ `useEffect(() => { fetch() }, [])` แบบเดิมๆ ถือว่าผิด pattern ในโปรเจกต์นี้

### React Query — ทำไมดีกว่า useEffect+fetch
React Query จัดการเรื่องน่าเบื่อให้หมด: loading state, error state, **cache** (ไม่ fetch ซ้ำถ้ามีข้อมูลอยู่แล้ว), **refetch อัตโนมัติ** (เช่น alert feed ที่ refresh ทุก 30 วินาที — ทำง่ายมากด้วย React Query)

```tsx
'use client';
function AlertFeed() {
  const { data, isLoading } = useQuery({
    queryKey: ['alerts'],
    queryFn: () => fetch('/api/v1/alerts').then(r => r.json()),
    refetchInterval: 30_000,   // refresh ทุก 30 วิ — แค่บรรทัดเดียว!
  });
  if (isLoading) return <Skeleton />;
  return <List items={data.items} />;
}
```

### โครงสร้าง dashboard
```
dashboard/
├── app/
│   ├── (auth)/         # หน้า login (วงเล็บ = "route group" ไม่โผล่ใน URL)
│   ├── (dashboard)/    # หน้าหลังบ้านทั้งหมด (machines, alerts, ...)
│   └── api/            # API route ของ Next เอง (ถ้าจำเป็น — ส่วนใหญ่เรียก backend ตรง)
├── components/         # component ที่ใช้ซ้ำ
└── lib/                # helper, config, api client
```

> 📌 **สถานะ:** หลายหน้าเสร็จแล้ว (machine list, detail, signatures, alerts, policy). i18n เต็มรูปแบบจะ polish ใน Phase 5

---

<a name="7"></a>
## 7. เจาะรายตัว #5 — PostgreSQL

### มันคืออะไร
ฐานข้อมูล SQL (relational) — **"แหล่งความจริงเดียว" (single source of truth)** ของทั้งระบบ ทุกอย่างที่ต้องเก็บถาวรอยู่ที่นี่ ถ้าคุณเคยใช้ MySQL/SQLite ใน CRUD มาก่อน concept เดียวกัน แต่ Postgres มีของเล่นมากกว่า

### ของใหม่ที่ Postgres มี (และโปรเจกต์ใช้จริง)

**(ก) JSONB — เก็บ JSON ในคอลัมน์เดียว**
- บางข้อมูลโครงสร้างไม่ตายตัว เช่น "chain ของใบรับรองดิจิทัล" หรือ "รายการ CPE ของ CVE" → เก็บเป็น JSONB ในคอลัมน์เดียวได้เลย ไม่ต้องสร้างตารางย่อย
- ต่างจาก JSON ธรรมดาตรงที่ JSONB **ค้นหา/index ได้** — เร็วกว่า

**(ข) Index — กุญแจของความเร็ว**
- ตาราง `software_records` จะมีถึง **500,000 แถว** (1,000 เครื่อง × 500 software). ถ้าไม่มี index การค้นหา "เครื่องไหนมี Chrome บ้าง" จะช้ามาก
- index = "สารบัญ" ของตาราง ทำให้ค้นเจอเร็วโดยไม่ต้องไล่อ่านทุกแถว
- โปรเจกต์ใช้ index หลายแบบ เช่น **GIN trigram** (ค้นหาชื่อแบบ fuzzy เช่นพิมพ์ "chrom" เจอ "Chrome")

**(ค) Materialized view — ตารางสรุปที่คำนวณไว้ล่วงหน้า**
- เช่น `license_installations` = "license แต่ละตัวถูกติดตั้งบนเครื่องไหนบ้าง" คำนวณจากการ join ตาราง license กับ software_records
- แทนที่จะ join ใหม่ทุกครั้งที่ถาม (ช้า) → คำนวณไว้ล่วงหน้าเก็บเป็น "view" แล้ว refresh เป็นช่วงๆ

### Convention ของตาราง (จาก `docs/03-data-model.md`)
- ชื่อตาราง: พหูพจน์ snake_case → `machines`, `software_records`
- ทุกตารางมี `id` (เลขรันใช้ภายใน) + `uuid` (ที่เปิดเผยใน API — กัน enumerate)
- **Soft delete** — ไม่ลบแถวจริง แต่ใส่ `deleted_at` (เพื่อเก็บ audit trail) สำหรับข้อมูลสำคัญ
- เวลาทุกตัวเก็บเป็น UTC (`TIMESTAMPTZ`)

> 💡 **คุณ "ไม่ได้เขียน SQL ตรงๆ"** ส่วนใหญ่ — คุณประกาศ models ใน SQLAlchemy (Python) แล้ว Alembic gen migration ให้ แต่คุณ **ต้องอ่าน SQL ออก** เพื่อ debug และต้องเข้าใจเรื่อง index เพื่อไม่ให้ระบบช้า

---

<a name="8"></a>
## 8. เจาะรายตัว #6 — Docker / Nginx (กาวที่ยึดทุกอย่าง)

### ปัญหา: 6 process จะรันพร้อมกันยังไง?
เรามี Postgres, Redis, Backend, Worker, Dashboard (+ Nginx ตอน prod) — 5-6 โปรแกรมที่ต้องรันพร้อมกันและคุยกันได้ ถ้าให้คุณลงเองทีละตัวบนเครื่อง = ฝันร้าย (เวอร์ชันชนกัน, ลืม config)

### Docker Compose มาแก้
**Docker** = ห่อแต่ละโปรแกรมไว้ใน "กล่อง" (container) ที่มีทุกอย่างที่มันต้องใช้ครบในตัว
**Docker Compose** = ไฟล์ `docker-compose.yml` ไฟล์เดียวที่บอกว่า "ระบบนี้มีกล่องอะไรบ้าง เชื่อมกันยังไง" แล้วสั่ง `docker compose up` ทีเดียวขึ้นทั้งระบบ

```
docker compose up   →   🐘 postgres   🔴 redis   🐍 backend   ⚙️ worker   ▲ dashboard
                        (ทุกตัวขึ้นพร้อมกัน คุยกันผ่าน network ภายในของ Docker)
```

- container คุยกันด้วย "ชื่อ" เช่น backend เชื่อม Postgres ที่ host ชื่อ `postgres:5432` (ไม่ใช่ `localhost`)
- network ภายใน Docker ถือว่า **trusted** (ดู `docs/02-architecture.md` — "ถ้า attacker เข้ามาในนี้ได้ = game over") เพราะงั้น Postgres/Redis ไม่เปิดออกเน็ตเลย

### Nginx — ยามหน้าประตู (เฉพาะตอน production)
จากเน็ตข้างนอก มีประตูเดียวคือ Nginx มันดู URL แล้วแจกงาน:
```
request เข้ามา → Nginx ดู path:
   /api/*  → ส่งไป Backend (FastAPI)
   /*      → ส่งไป Dashboard (Next.js)
```
และ Nginx จัดการ **HTTPS/TLS** (เข้ารหัสการเชื่อมต่อ) ให้ด้วย เพื่อให้ backend/dashboard ไม่ต้องยุ่งเรื่องนี้

> 📌 **สถานะ:** `docker-compose.yml` (dev) ใช้งานได้แล้วตั้งแต่ Phase 1. Nginx เป็น prod-only ดู `docs/08-deployment.md`

---

<a name="9"></a>
## 9. ทุกตัวทำงานด้วยกันยังไง — เดินตาม data flow จริง

ทฤษฎีพอแล้ว มาดูของจริง 3 สถานการณ์ เห็นภาพว่าทั้ง 7 ตัวเชื่อมกันยังไง

### 🎬 ฉากที่ 1: ติดตั้ง agent ใหม่บนเครื่องพนักงาน (Enrollment)
```
1. Admin เปิด Dashboard → สร้าง "enrollment token" (โค้ดใช้ครั้งเดียว อายุ 24 ชม.)
2. เอา token ไปติดตั้ง agent บนเครื่องพนักงาน
3. Agent → POST /api/v1/agents/enroll  (แนบ token + hostname, os, version)
4. Backend ตรวจ token → สร้างแถวใน `machines` → สร้าง "agent_token" ถาวร → คืนให้ agent
5. Agent เก็บ agent_token ไว้ในที่ปลอดภัยบนเครื่อง (Windows: %ProgramData% ที่เข้าได้เฉพาะ admin)
6. ต่อจากนี้ agent ใช้ token นี้ยืนยันตัวตนทุกครั้ง
```
→ เกี่ยวข้อง: **Agent → Nginx → Backend → Postgres**

### 🎬 ฉากที่ 2: Agent สแกนและส่งข้อมูล (หัวใจของระบบ)
```
1. ครบ 6 ชม. → Agent สแกนเครื่อง: อ่าน Registry เจอ software 200 ตัว + ตรวจลายเซ็นแต่ละตัว
2. Agent → POST /api/v1/agents/scans  (แนบ software 200 ตัว + ผลลายเซ็น, gzip บีบอัด)
3. Backend ตรวจ agent_token → เก็บลง Postgres (ตาราง scans, software_records, signature_records)
4. Backend โยน "ใบสั่งงาน" เข้าคิว Redis: ["จับคู่ alert", "จับคู่ CVE"]
5. Backend → ตอบ Agent กลับทันที: 202 Accepted  ✅ (Agent ไม่ต้องรอ!)
6. --- เบื้องหลัง (async) ---
7. Worker หยิบใบสั่งงานจาก Redis
8. Worker query Postgres → เทียบ software กับ blacklist/whitelist + ฐานข้อมูล CVE
9. Worker เจอของผิด → INSERT แถวใหม่ในตาราง `alerts`
```
→ เกี่ยวข้อง: **Agent → Backend → Postgres + Redis → Worker → Postgres** (ครบทุกตัวยกเว้น dashboard!)

> นี่คือจุดที่ concept "งานเบื้องหลัง" สำคัญ — step 5 ตอบเร็ว, step 6-9 ค่อยๆ ทำ ไม่มีใครรอ

### 🎬 ฉากที่ 3: Admin เปิด Dashboard ดูข้อมูล
```
1. Admin เปิดเบราว์เซอร์ → Next.js (Server Component) render หน้า
2. Next.js (ฝั่ง server) → GET /api/v1/machines → Backend
3. Backend query Postgres → คืน JSON รายการเครื่อง (+ จำนวน vuln, risk score)
4. Next.js เอาข้อมูลมา render เป็น HTML → ส่งให้เบราว์เซอร์ (ผู้ใช้เห็นข้อมูลทันที)
5. เบราว์เซอร์ "hydrate" (React เข้าควบคุม) → ต่อจากนี้ถ้ากดอะไร React Query จัดการ refetch
6. Alert feed ใช้ React Query refetch ทุก 30 วิ → เห็น alert ใหม่จากฉากที่ 2 โดยไม่ต้อง refresh
```
→ เกี่ยวข้อง: **Browser → Nginx → Dashboard → Backend → Postgres**

### สรุปเป็นภาพเดียว: "ใครเริ่มงาน"
```
┌─ เครื่องจักรเริ่ม ─────────────────┐   ┌─ คนเริ่ม ──────────────────┐
│ Agent (ทุก 6 ชม.) → scan          │   │ Admin → เปิด dashboard      │
│ ตัวจับเวลา (ทุกวัน) → CVE sync    │   │ Admin → กดสร้าง license     │
│        ↓                          │   │        ↓                    │
│   Backend รับ → Postgres          │   │   Dashboard → Backend       │
│        ↓                          │   │        ↓                    │
│   Redis queue → Worker ทำงานหนัก  │   │   Postgres → ตอบกลับ        │
└───────────────────────────────────┘   └─────────────────────────────┘
                  └──────── ทั้งคู่เขียนลง Postgres ตัวเดียวกัน ───────┘
```

---

<a name="10"></a>
## 10. Glossary — คำศัพท์ใหม่ที่จะเจอบ่อย

| คำ | แปลสั้นๆ | เจอที่ไหน |
|----|---------|----------|
| **Agent** | โปรแกรมเล็กๆ ที่รันบนเครื่องที่ถูกตรวจ | ทั้งระบบ |
| **Endpoint** | เครื่องปลายทาง (คอมพนักงาน) ที่ลง agent | overview |
| **Single binary** | ไฟล์โปรแกรมเดียวจบ ไม่ต้องลง runtime เพิ่ม | Go agent |
| **Heartbeat** | สัญญาณ "ฉันยังอยู่" ที่ agent ส่งทุก 60 วิ | protocol |
| **Enrollment** | ขั้นตอนลงทะเบียน agent ใหม่เข้าระบบ | protocol |
| **async/await** | ทำงานไม่บล็อก ระหว่างรอ I/O ไปทำอื่นก่อน | backend |
| **ORM** | เขียนโค้ดแทน SQL (SQLAlchemy/Prisma) | backend |
| **Migration** | ไฟล์บันทึกการเปลี่ยนโครงสร้าง DB ทีละขั้น | Alembic |
| **Schema (Pydantic)** | รูปร่าง JSON เข้า/ออก API | backend |
| **Model (SQLAlchemy)** | รูปร่างตารางใน DB | backend |
| **JWT** | token เข้ารหัสที่บอกว่า "นี่ใคร" หลัง login | auth |
| **bcrypt** | วิธี hash รหัสผ่าน (เก็บแบบกู้คืนไม่ได้) | auth |
| **Worker** | process แยกที่ทำงานหนัก/งานตามเวลา | arq |
| **Job queue** | คิวงานที่ backend หย่อนไว้ให้ worker หยิบ | Redis/arq |
| **Cache** | เก็บผลลัพธ์ชั่วคราวเพื่อความเร็ว | Redis |
| **Rate limit** | จำกัดจำนวนครั้งต่อเวลา (กัน abuse) | Redis |
| **RSC / Server Component** | component ที่ render บน server | Next.js |
| **Hydrate** | React เข้าควบคุม HTML ที่ server ส่งมา | Next.js |
| **JSONB** | คอลัมน์เก็บ JSON ที่ index ได้ | Postgres |
| **Index** | "สารบัญ" ของตาราง ทำให้ค้นเร็ว | Postgres |
| **Materialized view** | ตารางสรุปที่คำนวณไว้ล่วงหน้า | Postgres |
| **Soft delete** | ไม่ลบจริง แค่ใส่ `deleted_at` | Postgres |
| **CVE** | รหัสช่องโหว่ความปลอดภัยมาตรฐานสากล (CVE-2024-xxxx) | vuln module |
| **NVD / OSV** | ฐานข้อมูลช่องโหว่สาธารณะ (แหล่งดึง CVE) | worker |
| **CPE** | รหัสมาตรฐานระบุชื่อ software (ใช้จับคู่ CVE) | vuln module |
| **Authenticode / codesign** | ระบบลายเซ็นดิจิทัลของ Windows / macOS | agent |
| **Reverse proxy** | ตัวรับ request แล้วแจกไปปลายทาง (Nginx) | infra |
| **Container** | "กล่อง" ที่ห่อโปรแกรม + dependency (Docker) | infra |

---

<a name="11"></a>
## 11. แผนที่: ถ้าจะแตะ X ต้องเรียนรู้อะไรบ้าง

ตอนคุณจะลงมือเขียนโค้ดแต่ละส่วน นี่คือลำดับที่ควรเข้าใจ:

### ถ้าจะแตะ **Agent (Go)**
1. เรียน Go พื้นฐาน: struct, error handling, goroutine
2. อ่าน `docs/modules/01-agent.md` + `docs/05-agent-protocol.md`
3. เข้าใจ Windows Registry / macOS plist (ที่ที่ software ถูกบันทึก)
4. เข้าใจ Authenticode/codesign (การตรวจลายเซ็น)
> หัวใจ: "single binary, ห้าม depend runtime, รองรับ Win+Mac ตั้งแต่แรก"

### ถ้าจะแตะ **Backend (FastAPI)**
1. เรียน Python async (`async def`/`await`)
2. เข้าใจ 3 ชั้น: **model** (DB) → **schema** (API) → **service** (logic) → **router** (HTTP)
3. อ่าน `docs/04-api-contracts.md` (API หน้าตายังไง) + `docs/03-data-model.md` (ตาราง)
4. เรียน SQLAlchemy + Alembic (เขียน model → gen migration)
5. **เขียน test ก่อน (TDD)** โดยเฉพาะ logic เช่น CVE matching, risk score (กฎใน `CLAUDE.md`)
> หัวใจ: "logic อยู่ใน service ไม่ใช่ใน route, งานหนักโยนเข้า worker, ทุก I/O เป็น async"

### ถ้าจะแตะ **Worker**
1. เข้าใจ concept job queue ก่อน (ข้อ 5 ของเอกสารนี้)
2. เรียน `arq` (กำหนด task + schedule)
3. เข้าใจว่างานไหนควรเป็น worker (นาน / ตามเวลา) งานไหนควรอยู่ backend (เร็ว / ตอบทันที)

### ถ้าจะแตะ **Dashboard (Next.js)**
1. เข้าใจ Server Component vs Client Component (เมื่อไหร่ใช้ `'use client'`)
2. เรียน React Query (แทน useEffect+fetch)
3. อ่าน `docs/modules/07-dashboard.md` + `docs/09-coding-conventions.md`
4. ใช้ shadcn/ui + Tailwind, ทุกข้อความผ่าน next-intl (ห้าม hardcode)
> หัวใจ: "Server Components by default, ห้าม fetch ใน useEffect, i18n ทุก string"

### ลำดับการทำทั้งโปรเจกต์ (จาก `ROADMAP.md`)
```
Phase 1 ✅ Infra + Auth (เสร็จแล้ว)
Phase 2 ✅ Agent + Inventory (เสร็จ — agent เคยรันจริงเจอ 177 apps)
Phase 3 🔨 Signature ✅ + Unauthorized ✅ + CVE ⬅️ กำลังทำ Module 5
Phase 4 ⬜ License + Dashboard + Reporting
Phase 5 ⬜ User mgmt + i18n + Telemetry + Polish
```
**กฎเหล็ก:** ห้ามข้าม phase, ห้ามทำหลาย module พร้อมกัน, phase ก่อนต้อง test ผ่านก่อน

---

## 🎯 สรุปสั้นที่สุด (ถ้าจำได้แค่ย่อหน้าเดียว)

SoftSentry คือระบบที่ **agent (Go)** บนเครื่องพนักงานสแกน software ส่งให้ **backend (FastAPI)** ผ่าน **Nginx**, backend เก็บลง **Postgres** แล้วโยนงานหนักผ่านคิว **Redis** ให้ **worker (arq)** วิเคราะห์ช่องโหว่/แจ้งเตือนเบื้องหลัง, สุดท้าย IT admin เปิด **dashboard (Next.js)** มาดูผล ทุกอย่างรันเป็น container ผ่าน **Docker Compose**.

ต่างจาก Next CRUD ตรงที่: (1) งานเริ่มจากทั้งคนและเครื่องจักร (2) งานหนักแยกไปทำเบื้องหลัง (3) ใช้ 3 ภาษาตามความถนัดของแต่ละงาน (4) แยกเป็นหลาย process ที่คุยกันผ่าน network.

> อ่านจบแล้วลองกลับไปดูแผนภาพข้อ 2 อีกครั้ง — คราวนี้คุณจะเห็น "เรื่องราว" ไม่ใช่แค่กล่องกับลูกศร 🚀
