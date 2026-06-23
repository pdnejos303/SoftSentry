# syntax=docker/dockerfile:1.7
# ── SoftSentry Agent · builder image (runtime-build, ไม่ bake binary เข้า image) ──
# image นี้เตรียมแค่ "เครื่องมือ" (Go toolchain + rsrc) และ build-publish.sh
# **ไม่** copy source หรือ build binary ตอน image build — source ถูก bind-mount เข้ามา
# ตอน container รัน (ดู docker-compose.yml: ./agent:/src-ro:ro) แล้ว build-publish.sh
# คอมไพล์ + publish ลง named volume `agent_binaries` ที่ backend เสิร์ฟ
#
# ทำไมถึง build ตอน runtime ไม่ใช่ image build:
#   เดิม binary ถูก bake เข้า image → `docker compose up` เฉยๆ (ไม่มี --build) จะ
#   republish binary "เก่าที่ baked ไว้" ทับ volume เสมอ = stale-binary trap
#   ย้าย build มาตอน container รัน → ทุก `up` recompile จาก source ปัจจุบัน
#   ไม่ต้องพึ่ง --build อีก (image เปลี่ยนเฉพาะตอนแก้ toolchain/สคริปต์นี้)
FROM golang:1.25-alpine
WORKDIR /build
# agent ไม่มี cgo → cross-compile GOOS=windows จาก Linux ได้ตรงๆ
ENV CGO_ENABLED=0
# rsrc: เครื่องมือ pure-Go ฝัง Windows manifest เข้า .syso (ทำงานบน Linux ได้)
RUN go install github.com/akavel/rsrc@latest
# เก็บสำเนา build-publish.sh ไว้ใน image เป็น fallback (เผื่อรันโดยไม่ได้ bind-mount source)
COPY build-publish.sh /usr/local/bin/build-publish.sh
RUN chmod +x /usr/local/bin/build-publish.sh
# รันครั้งเดียวแล้ว exit 0 — backend รอด้วย depends_on: service_completed_successfully
# สำคัญ: รัน script จาก **source ที่ bind-mount (/src-ro)** ไม่ใช่สำเนา baked ในภาพ — มิฉะนั้น
# การแก้ build-publish.sh จะไม่มีผลจนกว่าจะ `--build` ใหม่ (= stale-script trap แบบเดียวกับ
# stale-binary) การรันจาก /src-ro ทำให้ทั้ง source และ orchestration สดตาม working tree เสมอ
CMD ["sh", "-c", "if [ -f /src-ro/build-publish.sh ]; then exec sh /src-ro/build-publish.sh; else exec /usr/local/bin/build-publish.sh; fi"]
