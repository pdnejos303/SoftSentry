# SoftSentry Agent

Single-binary cross-platform agent (Windows + macOS) written in Go 1.22.

## Phase 1 scope

- Cobra CLI skeleton: `enroll`, `run`, `scan`, `version`
- Config file (`~/.softsentry/config.yaml` or `%ProgramData%\SoftSentry\config.yaml`)
- Token storage (filesystem with restrictive ACL)
- HTTP transport to backend with bearer auth
- Health check `/heartbeat` once on `run`

Real scanning (Windows registry + macOS plist + Authenticode) lands in **Phase 2** per ROADMAP.

## Build

```bash
go build -o dist/softsentry-agent ./cmd/softsentry-agent

# Cross-compile
GOOS=windows GOARCH=amd64 go build -o dist/agent.exe ./cmd/softsentry-agent
GOOS=darwin  GOARCH=arm64 go build -o dist/agent-mac ./cmd/softsentry-agent
```

## Usage

```bash
# 1) admin generates enrollment token in dashboard, gives to agent host
softsentry-agent enroll --token <token> --server https://softsentry.example.com

# 2) agent runs on schedule (default 6h)
softsentry-agent run

# 3) one-off scan to stdout for debugging
softsentry-agent scan --output stdout
```

## Config

```yaml
# ~/.softsentry/config.yaml
server_url: https://softsentry.example.com
machine_uuid: 11111111-aaaa-...
scan_interval_hours: 6
auto_update_enabled: true
log_level: info
```

Token is stored separately in `~/.softsentry/token` (0600 on Unix, ACL admin-only on Windows).
