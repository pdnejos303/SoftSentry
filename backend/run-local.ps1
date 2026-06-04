# Run the SoftSentry backend locally with NO Docker / NO Postgres / NO Redis.
# Self-contained: SQLite file + in-memory Redis shim (see scripts/run_local.py).
#
#   .\run-local.ps1                 # serve on http://127.0.0.1:8000
#   .\run-local.ps1 -Port 8001      # different port
#   .\run-local.ps1 -Reset          # wipe the local SQLite DB first
#
# Open http://127.0.0.1:8000/docs and log in with the admin from ..\.env
# (INITIAL_ADMIN_EMAIL / INITIAL_ADMIN_PASSWORD).
param(
    [int]$Port = 8000,
    [switch]$Reset
)
$ErrorActionPreference = "Stop"
$py = Join-Path $PSScriptRoot ".venv\Scripts\python.exe"
if (-not (Test-Path $py)) { throw "venv python not found at $py" }
$args = @("-m", "scripts.run_local", "--port", $Port)
if ($Reset) { $args += "--reset" }
& $py @args
