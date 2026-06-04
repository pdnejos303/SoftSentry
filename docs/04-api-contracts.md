# 04 — API Contracts

> REST API ใช้ FastAPI. Base path: `/api/v1`. ทุก response เป็น JSON, ทุก auth ใช้ Bearer token

## Conventions

- **URL**: lowercase, plural noun (`/machines`, `/software`). Resource id ใช้ UUID
- **Method**: GET (read), POST (create), PATCH (partial update), PUT (full update — หลีกเลี่ยง), DELETE (soft delete)
- **Status codes**:
  - `200` success
  - `201` created
  - `204` no content
  - `400` validation error
  - `401` unauthenticated
  - `403` forbidden (RBAC)
  - `404` not found
  - `409` conflict (duplicate, etc.)
  - `422` semantic error (Pydantic)
  - `429` rate limited
  - `500` server error
- **Pagination**: `?page=1&page_size=50` (default 50, max 200). Response มี `{items, total, page, page_size, total_pages}`
- **Filter**: query params (`?status=online&os=windows`)
- **Sort**: `?sort=last_seen_at:desc,hostname:asc`
- **Search**: `?q=chrome` (free-text search ใน relevant field)
- **Errors**: `{ "error": { "code": "INVALID_VERSION", "message": "...", "field": "version" } }`
- **Timestamps**: ISO 8601 UTC (`2026-05-28T10:30:00Z`)

---

## Authentication

### `POST /api/v1/auth/login`
**Request**
```json
{ "email": "admin@corp.local", "password": "..." }
```
**Response 200**
```json
{
  "access_token": "eyJ...",
  "token_type": "bearer",
  "expires_in": 3600,
  "user": { "uuid": "...", "email": "...", "full_name": "...", "role": "admin" }
}
```
**Rate limit:** 5 attempts / 5 min / IP

### `POST /api/v1/auth/refresh`
Refresh access token (uses HTTP-only cookie refresh token)

### `POST /api/v1/auth/logout`
Revoke refresh token

### `GET /api/v1/auth/me`
Get current user info

---

## Agent endpoints

ทุก endpoint ใต้นี้ใช้ **agent token** (Bearer). ไม่ใช่ user JWT

### `POST /api/v1/agents/enroll`
**Headers:** `X-Enrollment-Token: <one-time-token>`
**Request**
```json
{
  "hostname": "DESKTOP-ABC123",
  "os": "windows",
  "os_version": "10.0.26200",
  "arch": "amd64",
  "agent_version": "1.0.0"
}
```
**Response 201**
```json
{
  "machine_uuid": "...",
  "agent_token": "<long-lived bearer token>",
  "config": { "scan_interval_hours": 6, "auto_update_enabled": true }
}
```

### `POST /api/v1/agents/heartbeat`
**Auth:** agent token
**Request**
```json
{ "agent_version": "1.0.0", "uptime_seconds": 86400 }
```
**Response 200**
```json
{ "config_changed": false, "manual_scan_requested": false, "agent_update_available": null }
```

### `GET /api/v1/agents/config`
**Auth:** agent token
**Response 200**
```json
{ "scan_interval_hours": 6, "auto_update_enabled": true, "manual_scan_requested": false }
```

### `POST /api/v1/agents/scans`
**Auth:** agent token
**Request**
```json
{
  "started_at": "...",
  "completed_at": "...",
  "scan_type": "auto",
  "software": [
    {
      "name": "Google Chrome",
      "version": "120.0.6099.130",
      "publisher": "Google LLC",
      "install_date": "2024-03-15",
      "install_path": "C:\\Program Files\\Google\\Chrome",
      "install_size_kb": 350000,
      "arch": "x64",
      "source": "registry",
      "signature": {
        "status": "valid",
        "signer": "Google LLC",
        "issuer": "DigiCert Trusted G4 Code Signing RSA4096 SHA384 2021 CA1",
        "cert_thumbprint": "abc...",
        "cert_valid_from": "2023-01-01",
        "cert_valid_to": "2026-01-01",
        "signature_algorithm": "SHA256-RSA",
        "chain": [ ... ]
      }
    }
  ]
}
```
**Response 202** (async processing)
```json
{ "scan_uuid": "...", "queued_for_analysis": true }
```

### `GET /api/v1/agents/binary/latest`
Auto-update: agent ดู version ใหม่.
**Response 200**
```json
{
  "version": "1.0.1",
  "platform": "windows",
  "arch": "amd64",
  "download_url": "/api/v1/agents/binary/download/1.0.1/windows/amd64",
  "sha256": "..."
}
```

---

## Machines (Dashboard endpoints)

### `GET /api/v1/machines`
**Auth:** JWT (any role)
**Query**: `q`, `status`, `os`, `tag`, `page`, `page_size`, `sort`
**Response 200**
```json
{
  "items": [
    {
      "uuid": "...",
      "hostname": "DESKTOP-ABC123",
      "os": "windows",
      "os_version": "10.0.26200",
      "agent_version": "1.0.0",
      "status": "online",
      "last_seen_at": "...",
      "last_scan_at": "...",
      "tags": ["finance", "tokyo"],
      "software_count": 152,
      "vulnerability_count": { "critical": 0, "high": 2, "medium": 5, "low": 1 },
      "risk_score": 21
    }
  ],
  "total": 1234, "page": 1, "page_size": 50, "total_pages": 25
}
```

### `GET /api/v1/machines/{uuid}`
**Response 200**: full machine detail

### `PATCH /api/v1/machines/{uuid}`
**Auth:** admin
**Request**: `{ "tags": ["finance", "vip"] }` (only tags editable in v1)

### `DELETE /api/v1/machines/{uuid}`
**Auth:** admin. Soft delete. Agent ถัดไปจะถูก reject token

### `POST /api/v1/machines/{uuid}/trigger-scan`
**Auth:** admin. Toggle `manual_scan_requested=true`. Agent จะ scan ใน heartbeat ถัดไป

### `GET /api/v1/machines/{uuid}/software`
Software list ของเครื่องนี้.
**Query:** `q`, `signature_status`, `has_vulnerability`, `page`...

### `GET /api/v1/machines/{uuid}/history`
Install/uninstall events.

### `GET /api/v1/machines/{uuid}/scans`
Scan history.

---

## Software (cross-machine)

### `GET /api/v1/software`
**Query**: `q`, `publisher`, `signature_status`, `installed_on_machine_uuid`, `page`...
**Response 200** — aggregate view: 1 row per unique (name, version)
```json
{
  "items": [
    {
      "name": "Google Chrome",
      "version": "120.0.6099.130",
      "publisher": "Google LLC",
      "installed_count": 47,
      "machines": ["uuid1", "uuid2", ...],   // limit 10, full list ใน detail
      "signature_status": "valid",
      "vulnerabilities": { "critical": 0, "high": 1 }
    }
  ]
}
```

### `GET /api/v1/software/top`
**Query:** `?limit=10`
Top N software ที่ install เยอะที่สุด

### `POST /api/v1/software/compare`
**Request**
```json
{ "machine_uuids": ["uuid-A", "uuid-B"] }
```
**Response 200**
```json
{
  "common": [ { "name": "...", "version": "..." } ],
  "only_in_a": [ ... ],
  "only_in_b": [ ... ],
  "version_diff": [ { "name": "...", "version_a": "1.0", "version_b": "2.0" } ]
}
```

---

## Signatures

### `GET /api/v1/signatures`
**Query**: `status` (`valid`/`expired`/`invalid`/`unsigned`), `publisher`, ...

### `GET /api/v1/signatures/{software_uuid}`
Certificate chain detail

---

## Whitelist / Blacklist

### `GET|POST|PATCH|DELETE /api/v1/whitelist`
**Auth:** admin (write), any (read)

### `POST /api/v1/whitelist/bulk`
**Request**: multipart/form-data — CSV file (columns: `name_pattern,publisher_pattern,version_constraint,notes`)
**Response 200**
```json
{ "added": 25, "skipped": 2, "errors": [{"row": 7, "reason": "..."}] }
```

### `GET|POST|PATCH|DELETE /api/v1/blacklist`
Same shape + `severity`, `reason` fields

---

## Vulnerabilities

### `GET /api/v1/vulnerabilities`
**Query**: `severity`, `machine_uuid`, `cve_id`, `is_dismissed`, ...
**Response 200**
```json
{
  "items": [
    {
      "uuid": "...",
      "cve_id": "CVE-2024-12345",
      "severity": "high",
      "cvss_score": 7.5,
      "description": "...",
      "affected_software": [
        { "machine_uuid": "...", "hostname": "...", "name": "Chrome", "version": "..." }
      ],
      "recommended_version": "120.0.6099.150",
      "is_dismissed": false
    }
  ]
}
```

### `POST /api/v1/vulnerabilities/{uuid}/dismiss`
**Auth:** admin
**Request**: `{ "reason": "Accepted risk, internal app" }`

### `GET /api/v1/cve/{cve_id}`
Full CVE detail (from local cve_records cache)

---

## Licenses

### `GET|POST|PATCH|DELETE /api/v1/licenses`
**Auth:** admin (write), any (read)

### `GET /api/v1/licenses/{uuid}/installations`
รายการเครื่องที่ติดตั้ง software ของ license นี้

### `GET /api/v1/licenses/compliance-summary`
**Response 200**
```json
{
  "total_licenses": 23,
  "compliant": 18,
  "over_used": 3,
  "expiring_30d": 2,
  "expiring_60d": 4,
  "expiring_90d": 5,
  "expired": 0,
  "compliance_rate": 0.78
}
```

---

## Alerts

### `GET /api/v1/alerts`
**Query**: `status`, `severity`, `type`, `machine_uuid`, ...

### `POST /api/v1/alerts/{uuid}/acknowledge`
**Auth:** admin

### `POST /api/v1/alerts/{uuid}/resolve`
**Auth:** admin

---

## Dashboard

### `GET /api/v1/dashboard/overview`
**Response 200**
```json
{
  "machines": { "total": 1234, "online": 1100, "offline": 134 },
  "software": { "total_unique": 4567, "total_installations": 89012 },
  "vulnerabilities": { "critical": 5, "high": 23, "medium": 87, "low": 145 },
  "alerts": { "open": 12, "acknowledged": 3 },
  "licenses": { "compliant": 18, "over_used": 3, "expiring_30d": 2 }
}
```

### `GET /api/v1/dashboard/risk-scores`
**Query:** `?limit=10` (top N risky machines)

### `GET /api/v1/dashboard/charts/vuln-trend`
**Query:** `?period=30d`

---

## Reports

### `POST /api/v1/reports/generate`
**Auth:** admin
**Request**
```json
{ "type": "org_summary" | "machine_detail", "machine_uuid": "...", "format": "pdf" | "csv" }
```
**Response 202**
```json
{ "report_uuid": "...", "status": "queued" }
```

### `GET /api/v1/reports/{uuid}`
Poll status + download URL when ready

### `GET /api/v1/reports/{uuid}/download`
Binary download

### `GET|POST|PATCH|DELETE /api/v1/reports/schedules`
Recurring report schedules

---

## Users (Admin only)

### `GET /api/v1/users`
### `POST /api/v1/users`
### `PATCH /api/v1/users/{uuid}`
### `DELETE /api/v1/users/{uuid}` — soft delete + revoke tokens

---

## Audit Log

### `GET /api/v1/audit-logs`
**Auth:** admin
**Query:** `user_uuid`, `action`, `entity_type`, `from`, `to`, ...

---

## OpenAPI

FastAPI generate Swagger UI at `/api/v1/docs` (dev only — disable ใน prod ผ่าน env)
