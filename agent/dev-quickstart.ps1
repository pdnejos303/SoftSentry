<#
  SoftSentry Agent — Dev Quickstart (กดทีเดียวจบ)

  ทำให้ครบในคำสั่งเดียว: login admin -> ขอ enrollment token -> enroll เครื่องนี้ -> start agent
  ใช้สำหรับ "ทดลองบนเครื่อง Dev" เท่านั้น (รัน foreground, ไม่ต้องสิทธิ์ admin)

  วิธีใช้ (เปิด PowerShell ในโฟลเดอร์ agent):
      .\dev-quickstart.ps1                 # enroll (ถ้ายังไม่เคย) แล้วรัน agent
      .\dev-quickstart.ps1 -Reenroll       # บังคับ enroll ใหม่ (ออก token ใบใหม่)

  สำหรับเครื่อง endpoint จริง (production) ให้ใช้ `install` เป็น Windows Service แทน:
      .\softsentry-agent.exe install --enrollment-token <tok> --server http://<server-ip>:47800
#>
param(
    # backend ของ SoftSentry map อยู่ที่ host port 47800 (ดู docker-compose.yml / .env)
    [string]$Server   = "http://localhost:47800",
    [string]$Email    = "admin@local",
    [string]$Password = "ChangeMe!2026",
    [switch]$Reenroll
)
$ErrorActionPreference = "Stop"
$exe = Join-Path $PSScriptRoot "softsentry-agent.exe"
$api = "$Server/api/v1"

if (-not (Test-Path $exe)) {
    throw "ไม่พบ $exe — build ก่อนด้วย: go build -o softsentry-agent.exe ./cmd/softsentry-agent"
}

# 0) backend ติดไหม
try { Invoke-RestMethod "$Server/health" -TimeoutSec 5 | Out-Null }
catch { throw "ต่อ backend ที่ $Server ไม่ได้ — สั่ง `docker compose up -d` ก่อน แล้วเช็คว่า backend map port ตรง" }

# 1) ถ้า enroll แล้วและไม่ได้สั่ง -Reenroll ก็ข้ามไปรันเลย
$alreadyEnrolled = (& $exe status 2>&1 | Select-String "Enrolled: yes") -ne $null
if ($alreadyEnrolled -and -not $Reenroll) {
    Write-Host "เครื่องนี้ enroll ไว้แล้ว — ข้ามไปสตาร์ต agent (ใช้ -Reenroll ถ้าต้องการ enroll ใหม่)" -ForegroundColor Yellow
} else {
    Write-Host "[1/3] login admin ..." -ForegroundColor Cyan
    $login = Invoke-RestMethod -Method Post "$api/auth/login" -ContentType application/json `
        -Body (@{ email = $Email; password = $Password } | ConvertTo-Json)

    Write-Host "[2/3] ขอ enrollment token (ใช้ครั้งเดียว) ..." -ForegroundColor Cyan
    $tok = Invoke-RestMethod -Method Post "$api/agents/enrollment-tokens" `
        -Headers @{ Authorization = "Bearer $($login.access_token)" } -ContentType application/json `
        -Body (@{ label = $env:COMPUTERNAME; expires_in_seconds = 86400 } | ConvertTo-Json)

    Write-Host "[3/3] enroll เครื่องนี้ ..." -ForegroundColor Cyan
    & $exe enroll --token $tok.token --server $Server
}

Write-Host "`nสตาร์ต agent (สแกนรอบแรกทันที + heartbeat ทุก 60s). กด Ctrl+C เพื่อหยุด`n" -ForegroundColor Green
& $exe run
