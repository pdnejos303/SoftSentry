# วิธีรัน SoftSentry (How to Run)

> เปิด Docker แล้วรันทั้งระบบด้วย **คำสั่งเดียว** — ไม่ต้องลง Python / Node / Postgres เองในเครื่อง
> สำหรับ dev แบบลงมือแก้ทีละ service ดู [`07-dev-setup.md`](07-dev-setup.md)

---

## ⚡ TL;DR — รันทั้งหมดด้วย Docker

```bash
# 1. มี .env แล้ว (ถ้ายังไม่มี: cp .env.example .env แล้วเติม secret จริง)
# 2. รันทุกอย่าง
docker compose up -d --build
```

แค่นี้ — backend, worker, dashboard, postgres, redis ขึ้นครบ

**เปิดเครื่องนี้ครั้งแรกก็ใช้ได้เลย** เพราะ:
- backend container รัน `alembic upgrade head` + `python -m app.seed` (สร้าง admin) **อัตโนมัติ** ก่อน serve
- DB schema + initial admin ถูก setup ให้เอง ไม่ต้อง migrate มือ
- WeasyPrint native libs (Pango/cairo/fonts-thai) อยู่ใน backend image แล้ว → PDF report ออกได้

> รอบแรก build image ช้า (~2–5 นาที). รอบถัดไปใช้ cache เร็ว

---

## เข้าใช้งานที่ไหน

| Service | URL (เครื่องนี้) | URL (default) |
|---------|------------------|---------------|
| Dashboard | http://localhost:3000 | http://localhost:3000 |
| Backend API | http://localhost:**8001** | http://localhost:8000 |
| Swagger docs | http://localhost:8001/docs | http://localhost:8000/docs |
| Postgres | localhost:5432 | localhost:5432 |
| Redis | localhost:**6380** | localhost:6379 |

> ⚠️ **ทำไม backend = 8001 / redis = 6380 บนเครื่องนี้?**
> `docker-compose.override.yml` (โหลดอัตโนมัติ) remap host port เพราะเครื่องนี้มีโปรเจกต์อื่นจอง 8000/6379 อยู่.
> port ภายใน container ไม่เปลี่ยน — service ยังคุยกันผ่าน `backend:8000` / `redis:6379` / `postgres:5432`.
> ถ้าเอาไปรันเครื่องอื่นที่ port ว่าง ให้ลบ `docker-compose.override.yml` ออก → กลับไปใช้ 8000/6379

### Login ครั้งแรก
ใช้ค่าจาก `.env`:
```
INITIAL_ADMIN_EMAIL=admin@local
INITIAL_ADMIN_PASSWORD=ChangeMe!2026
```
(seed script รันให้ครั้งเดียวตอน backend boot — idempotent, รันซ้ำไม่พัง)

---

## คำสั่งที่ใช้บ่อย

```bash
docker compose up -d --build          # build + start ทั้งหมด (เบื้องหลัง)
docker compose up -d                  # start (ไม่ rebuild)
docker compose ps                     # ดูสถานะ service
docker compose logs -f backend        # ดู log backend (Ctrl+C ออก)
docker compose logs -f worker         # log worker (CVE sync, report generation)
docker compose restart backend        # restart service เดียว
docker compose down                   # หยุดทั้งหมด (เก็บ data)
docker compose down -v                # หยุด + ลบ volume (ล้าง DB/redis/reports หมด)
```

### Hot-reload ตอน dev
`docker-compose.yml` mount `./backend/app` เข้า container อยู่แล้ว — แก้โค้ด backend แล้ว uvicorn reload ให้เอง
(แก้ dashboard ต้อง `docker compose up -d --build dashboard` ใหม่ เพราะ Next.js build เป็น production image)

---

## Observability (Prometheus + Grafana) — optional

ไม่ขึ้นโดย default. เปิดด้วย profile:

```bash
docker compose --profile observability up -d
```

| Service | URL | Login |
|---------|-----|-------|
| Prometheus | http://localhost:9090 | — |
| Grafana | http://localhost:3001 | admin / `GF_SECURITY_ADMIN_PASSWORD` ใน `.env` |

---

## Agent (รันแยก — เป็น Go binary บนเครื่อง endpoint)

agent **ไม่ได้อยู่ใน docker compose** เพราะมันต้องไปติดตั้งบนเครื่องพนักงานจริง (Windows/macOS) เพื่อสแกน software ในเครื่องนั้น:

```bash
cd agent
go run ./cmd/softsentry-agent enroll --token <enrollment-token-จาก-dashboard>
go run ./cmd/softsentry-agent run

# หรือ build เป็น binary แจกจ่าย
GOOS=windows GOARCH=amd64 go build -o dist/softsentry-agent.exe ./cmd/softsentry-agent
GOOS=darwin  GOARCH=arm64 go build -o dist/softsentry-agent     ./cmd/softsentry-agent
```

---

## Troubleshoot

| อาการ | สาเหตุ / วิธีแก้ |
|-------|-----------------|
| `port is already allocated` | port host ชนโปรเจกต์อื่น → แก้ `docker-compose.override.yml` เปลี่ยน host port |
| backend `unhealthy` / login ไม่ได้ | ดู `docker compose logs backend` — มักเป็น migrate fail หรือ `JWT_SECRET` ว่าง |
| dashboard เรียก API ไม่ได้ (CORS/connection refused) | `NEXT_PUBLIC_API_URL` ถูก bake ตอน build — ต้องตรงกับ host port ของ backend (เครื่องนี้ = 8001). แก้แล้วต้อง `--build dashboard` ใหม่ |
| PDF report เป็น `failed` | ปกติเฉพาะตอนรัน backend นอก Docker บน Windows (ไม่มี Pango/cairo). ใน container ออกได้ |
| อยาก reset ทุกอย่าง | `docker compose down -v` แล้ว `docker compose up -d --build` ใหม่ |

---

## หมายเหตุ Production

compose ชุดนี้สำหรับ **dev**. Production (TLS/nginx, secrets manager, backup) ดู [`08-deployment.md`](08-deployment.md)
- ⚠️ เปลี่ยน `JWT_SECRET`, `LICENSE_KEY_ENCRYPTION_KEY`, password ทั้งหมด — อย่าใช้ค่า dev
- ⚠️ อย่า commit `.env` จริงเข้า git
