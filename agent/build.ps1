<#
  SoftSentry Agent — build + publish (Windows)

  สิ่งที่ทำให้ครบในคำสั่งเดียว:
    1) go build ฝัง Version จริงเข้า binary (ldflags) — ไม่ใช่ค่า default 0.1.0 ที่ทำให้ update วนลูป
    2) คำนวณ SHA-256
    3) เขียน manifest.json ที่ version "ตรงกับ" binary เป๊ะ ลงในโฟลเดอร์ที่ backend เสิร์ฟ (agent_binary_dir)

  ใช้:
    .\build.ps1                       # build version 0.1.0 -> ..\backend\agent-binaries
    .\build.ps1 -Version 1.0.0        # ออก release ใหม่ (ต้องมากกว่าตัวเดิม agent ถึงจะ auto-update)
    .\build.ps1 -OutDir D:\path       # เขียนไปโฟลเดอร์อื่น

  หลัง build เสร็จ ถ้า backend รันใน docker อยู่แล้ว ไฟล์จะถูก mount เข้า /app/agent-binaries
  อัตโนมัติ (ดู docker-compose.yml) — ไม่ต้อง rebuild image
#>
param(
    [string]$Version = "0.1.0",
    [string]$OutDir  = (Join-Path $PSScriptRoot "..\backend\agent-binaries"),
    [string]$Arch    = "amd64"
)
$ErrorActionPreference = "Stop"

$pkg     = "github.com/softsentry/agent/internal/transport"
$ldflags = "-X $pkg.Version=$Version"
$exeName = "softsentry-agent-$Version-windows-$Arch.exe"

if (-not (Test-Path $OutDir)) { New-Item -ItemType Directory -Force $OutDir | Out-Null }
$exePath = Join-Path $OutDir $exeName

Write-Host "[1/3] build windows/$Arch (version $Version) ..." -ForegroundColor Cyan
$env:GOOS = "windows"; $env:GOARCH = $Arch
& go build -ldflags $ldflags -o $exePath ./cmd/softsentry-agent
if ($LASTEXITCODE -ne 0) { throw "go build failed" }

Write-Host "[2/3] sha256 ..." -ForegroundColor Cyan
$sha = (Get-FileHash -Algorithm SHA256 $exePath).Hash.ToLower()

Write-Host "[3/3] write manifest.json ..." -ForegroundColor Cyan
$manifest = [ordered]@{
    binaries = @(
        [ordered]@{
            version  = $Version
            os       = "windows"
            arch     = $Arch
            filename = $exeName
            sha256   = $sha
        }
    )
}
$manifestPath = Join-Path $OutDir "manifest.json"
# WriteAllText writes UTF-8 *without* BOM. (Out-File -Encoding utf8 on Windows
# PowerShell 5.1 prepends a BOM, which the backend's json.loads cannot parse →
# "No agent binary available" 503.)
$json = $manifest | ConvertTo-Json -Depth 5
[System.IO.File]::WriteAllText($manifestPath, $json, [System.Text.UTF8Encoding]::new($false))

Write-Host "`n✓ done" -ForegroundColor Green
Write-Host "  binary : $exePath"
Write-Host "  sha256 : $sha"
Write-Host "  manifest: $manifestPath  (version $Version)"
Write-Host "`nbinary ฝัง Version=$Version แล้ว — ตรงกับ manifest → ไม่มี update loop" -ForegroundColor Yellow
