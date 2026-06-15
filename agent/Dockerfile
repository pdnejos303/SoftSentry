# syntax=docker/dockerfile:1.7
# ── SoftSentry Agent · cross-compile + publish ───────────────────────────────
# Build agent binary (windows/amd64) ตอน `docker compose build` แล้ว copy ลง
# named volume ที่ backend mount เป็น agent_binary_dir — ไม่ต้องเก็บ .exe ใน git
#
# VERSION ถูก stamp เข้า binary (ldflags) และเขียนลง manifest.json ให้ "ตรงกัน"
# เป๊ะ — ถ้าไม่ตรงจะเกิด agent update-loop (ดู internal/transport/client.go)
ARG VERSION=0.1.0

FROM golang:1.25-alpine AS build
WORKDIR /src
ARG VERSION
# agent ไม่มี cgo → cross-compile GOOS=windows จาก Linux ได้ตรงๆ
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
# rsrc: เครื่องมือ pure-Go สำหรับฝัง Windows manifest เข้า .syso (ทำงานบน Linux ได้)
RUN go install github.com/akavel/rsrc@latest
COPY . .
RUN set -eu; \
    mkdir -p /out; \
    # ฝัง application manifest (เปิด TaskDialog/DPI + asInvoker) เข้า package dir
    # ก่อน build — ชื่อ *_windows_amd64.syso ทำให้ลิงก์เฉพาะ build Windows/amd64
    /go/bin/rsrc -manifest cmd/softsentry-agent/app.manifest -arch amd64 \
      -o cmd/softsentry-agent/rsrc_windows_amd64.syso; \
    GOOS=windows GOARCH=amd64 go build \
      -ldflags "-X github.com/softsentry/agent/internal/transport.Version=${VERSION}" \
      -o "/out/softsentry-agent-${VERSION}-windows-amd64.exe" ./cmd/softsentry-agent; \
    cd /out; \
    SHA=$(sha256sum "softsentry-agent-${VERSION}-windows-amd64.exe" | cut -d' ' -f1); \
    printf '{\n  "binaries": [\n    {"version": "%s", "os": "windows", "arch": "amd64", "filename": "softsentry-agent-%s-windows-amd64.exe", "sha256": "%s"}\n  ]\n}\n' \
      "${VERSION}" "${VERSION}" "${SHA}" > manifest.json

# publish stage: หน้าที่เดียวคือ copy artifact ลง volume /export แล้ว exit 0
# backend รอ stage นี้จบด้วย `depends_on: condition: service_completed_successfully`
FROM alpine:3.20 AS publish
COPY --from=build /out /artifacts
CMD ["sh", "-c", "cp -f /artifacts/* /export/ && echo 'published agent binaries:' && ls -l /export"]
