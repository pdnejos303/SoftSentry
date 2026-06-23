#!/bin/sh
# ── SoftSentry Agent · build + publish (รันตอน container start, ไม่ใช่ image build) ──
# agent-builder bind-mount source (read-only) เข้ามาที่ /src-ro แล้วสคริปต์นี้:
#   1) ตรวจว่ามี binary เดิมใน /export (volume ที่ backend เสิร์ฟ) หรือยัง
#   2) cross-compile binary ปัจจุบันจาก source ที่ mount มา (windows/amd64)
#   3) เทียบ sha กับของเดิม → บอกชัดว่า "เหมือนเดิม / เก่า(ต้องอัปเดต) / ยังไม่มี"
#   4) ลงทับเสมอ (ลบทุก version เก่า + manifest แล้ว copy ตัวใหม่) ให้ volume สะท้อน
#      source ปัจจุบันเป๊ะ — กัน binary เก่าค้าง และกันเคส "version เดิมแต่โค้ดเปลี่ยน"
#
# เพราะ build เกิดตอน "container รัน" (ไม่ใช่ตอน image build) ทุก `docker compose up`
# จึง recompile จาก source ล่าสุดเสมอ → ไม่ต้องสั่ง `--build` ก็ได้ binary ใหม่
set -eu

VERSION="${AGENT_VERSION:-0.1.0}"
EXE="softsentry-agent-${VERSION}-windows-amd64.exe"
# build stamp = UTC timestamp ของ build รอบนี้ — เปลี่ยนทุกครั้งที่ build, ใช้เป็น
# "ground truth" เทียบความสด: bake เข้า binary (โชว์บนหน้าแรก wizard) + เขียนลง manifest
# (โชว์บนหน้า deploy) ถ้าสองที่ตรงกัน = ตัวที่โหลด = ตัวที่ build จาก source ล่าสุดแน่นอน
BUILDSTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
echo "==> SoftSentry agent build-publish (version=${VERSION} build=${BUILDSTAMP})"

# 1) ตรวจ volume ก่อน — มีของเดิมไหม
echo "==> [1/4] checking existing binaries in /export:"
if ls /export/softsentry-agent-*.exe >/dev/null 2>&1; then
    ls -l /export/softsentry-agent-*.exe
else
    echo "    (none — first publish, will build into the volume)"
fi

# 2) build ปัจจุบันจาก source ที่ bind-mount มา (read-only) → copy ไป writable dir ก่อน
#    เพื่อไม่เขียน .syso ทับ host working tree และกัน permission issue ข้าม platform
echo "==> [2/4] compiling current source (windows/amd64)..."
rm -rf /build && cp -a /src-ro /build && cd /build
go mod download
# ฝัง Windows application manifest (DPI + asInvoker) เข้า .syso ก่อน build
/go/bin/rsrc -manifest cmd/softsentry-agent/app.manifest -arch amd64 \
    -o cmd/softsentry-agent/rsrc_windows_amd64.syso
mkdir -p /out
GOOS=windows GOARCH=amd64 go build \
    -ldflags "-X github.com/softsentry/agent/internal/transport.Version=${VERSION} -X github.com/softsentry/agent/internal/transport.BuildStamp=${BUILDSTAMP}" \
    -o "/out/${EXE}" ./cmd/softsentry-agent
NEWSHA=$(sha256sum "/out/${EXE}" | cut -d' ' -f1)
echo "    built ${EXE} sha=${NEWSHA}"

# 3) เทียบกับ binary เดิมใน volume → รายงานสถานะ
echo "==> [3/4] comparing against volume:"
if [ -f "/export/${EXE}" ]; then
    OLDSHA=$(sha256sum "/export/${EXE}" | cut -d' ' -f1)
    if [ "$OLDSHA" = "$NEWSHA" ]; then
        echo "    volume already has identical ${EXE} — refreshing anyway (idempotent)"
    else
        echo "    volume has OUTDATED ${EXE} (old=${OLDSHA}) — will overwrite"
    fi
else
    echo "    no matching ${EXE} in volume — will publish fresh"
fi

# 4) ลงทับเสมอ: ลบทุก binary/manifest เก่า แล้ว copy ตัวใหม่ + เขียน manifest ให้ตรง
#    (manifest version ต้อง match binary ที่ stamp ไว้ ไม่งั้น agent เข้า update-loop)
echo "==> [4/4] publishing to /export:"
rm -f /export/softsentry-agent-*.exe /export/manifest.json
cp "/out/${EXE}" /export/
printf '{\n  "binaries": [\n    {"version": "%s", "os": "windows", "arch": "amd64", "filename": "%s", "sha256": "%s", "build_stamp": "%s"}\n  ]\n}\n' \
    "${VERSION}" "${EXE}" "${NEWSHA}" "${BUILDSTAMP}" > /export/manifest.json
ls -l /export
echo "==> done."
