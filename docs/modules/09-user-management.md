# Module 9 — User & Access Management

## Purpose

- ผู้ใช้งาน dashboard (admin + viewer)
- Role-based access control (RBAC)
- Audit log — ใครทำอะไรเมื่อไหร่
- Login + password management

## Phase
Phase 5

## Depends on
- Module 7 (Dashboard) — UI host pages
- Phase 1 auth backbone

---

## Features & Acceptance Criteria

### 9.1 Login flow
**UI:** `/login`

**Fields:**
- email
- password
- "Remember me" toggle (extend refresh token TTL)

**Errors:**
- Wrong credentials: "Incorrect email or password" (don't leak which)
- Account locked (5 failed attempts): "Account temporarily locked. Try in 15 min"
- Inactive: "Account is disabled"

**Acceptance:**
- Valid creds → redirect `/` + JWT in memory + refresh cookie
- 5 wrong attempts in 5 min → lockout 15 min (rate limit by IP + email combo)
- Remember-me: refresh token TTL = 30 days (default), w/o = 24h

### 9.2 Password reset
**Self-service via email link (Phase 5+ — needs email setup)**

**Flow:**
1. User clicks "Forgot password" on login
2. Enter email → POST `/auth/password-reset-request`
3. Backend: if user exists, generate reset_token (random, 1h expiry), store hash, email link
4. User clicks link → `/reset-password?token=...`
5. Enter new password → POST `/auth/password-reset-confirm`
6. Backend: validate token, update password_hash, revoke all sessions, redirect login

**v1 fallback** (no email setup): admin reset via user management page

**Acceptance:**
- Wrong email → no error revealing existence (always show success message)
- Used/expired token → "Link expired or used"
- New password subject to policy (12 char min, complexity)

### 9.3 Profile page
**UI:** `/settings/profile`

**Fields:**
- Email (read-only — change requires admin)
- Full name (editable)
- Change password (current + new + confirm)
- Language preference (ไทย/EN)
- Theme: light/dark (Phase 5)

**Acceptance:**
- Save → reflect immediately
- Change password → invalidate all other sessions

### 9.4 User CRUD (admin only)
**UI:** `/settings/users`

**Table columns:**
- Email
- Full name
- Role (admin/viewer)
- Status (active/disabled)
- Last login
- Created at
- Actions: Edit, Disable, Delete (soft)

**Create form:**
- Email (validate format + uniqueness)
- Full name
- Role
- Initial password (random generated, displayed once + copy button)

**Edit:**
- Change role, full name, status
- "Reset password" button → generate new random + display once + force change on next login

**Delete (soft):**
- Confirm dialog
- Sets `deleted_at` + revokes all sessions
- Email becomes reusable (case: someone leaves company, new join uses same email)

**Acceptance:**
- Admin can CRUD all users
- Viewer sees disabled menu entry
- Cannot delete self
- Cannot demote last admin
- All actions logged in audit_log

### 9.5 Role-based access control
**Roles:**
- `admin` — full access
- `viewer` — read-only

**Backend enforcement:** `Depends(require_role('admin'))` on mutation endpoints

**Frontend enforcement:**
- Hide action buttons based on role
- Server still validates (frontend is convenience)

**Acceptance:**
- Viewer logs in → no "Delete" / "Edit" / "Add" buttons visible
- Viewer hits mutation endpoint directly → 403
- Admin can do everything

### 9.6 Audit log
**UI:** `/settings/audit-log` (admin only)

**Display:**
- Timestamp
- User (or "system" if automated)
- Action (formatted: `user.create`, `whitelist.delete`, etc.)
- Entity (type + id, clickable if still exists)
- Changes (before/after, JSON diff view)
- IP address
- User agent (truncated)

**Filters:**
- User
- Action type (dropdown of distinct values)
- Entity type
- Date range

**Pagination:** 50/page, default sort newest first

**Acceptance:**
- ทุก mutation operation logged
- Login success + failure logged
- Audit list paginated + filtered
- JSON diff readable (highlight changed fields)

### 9.7 Session management
**UI:** `/settings/profile` → tab "Active sessions"

**Display:**
- Each active session: device (parsed from user_agent), IP, last activity
- "Revoke" button per session
- "Revoke all other sessions" master button
- Current session marked "This device"

**Acceptance:**
- Login from new browser → appears in list
- Revoke → that token immediately invalid
- Revoke all → other sessions logged out within 30s (token check fails)

### 9.8 Password policy
- Min length 12
- Must contain: uppercase + lowercase + digit + special
- Cannot be in top-10k common passwords list (bundled)
- Cannot match previous 5 passwords (history table optional v1)

**Acceptance:**
- Weak password → reject with clear message which rule failed
- Common password ("Password123!") → reject

---

## API endpoints

```
# Auth
POST   /api/v1/auth/login
POST   /api/v1/auth/logout
POST   /api/v1/auth/refresh
GET    /api/v1/auth/me
POST   /api/v1/auth/password-reset-request
POST   /api/v1/auth/password-reset-confirm
POST   /api/v1/auth/change-password

# Users (admin)
GET    /api/v1/users
POST   /api/v1/users
GET    /api/v1/users/{uuid}
PATCH  /api/v1/users/{uuid}
DELETE /api/v1/users/{uuid}
POST   /api/v1/users/{uuid}/reset-password

# Profile (self)
PATCH  /api/v1/profile
GET    /api/v1/profile/sessions
DELETE /api/v1/profile/sessions/{uuid}
POST   /api/v1/profile/sessions/revoke-others

# Audit log (admin)
GET    /api/v1/audit-logs
GET    /api/v1/audit-logs/export?format=csv
```

---

## DB tables

- `users` (see [data-model](../03-data-model.md))
- `refresh_tokens`
  - id, user_id, token_hash, device_label, ip_address, user_agent, created_at, expires_at, revoked_at
- `password_reset_tokens`
  - id, user_id, token_hash, created_at, expires_at, used_at
- `audit_logs` (already designed)
- `password_history` (optional v1)

---

## Audit log conventions

**Action naming:** `<entity>.<verb>`
- `user.create`, `user.update`, `user.delete`, `user.disable`, `user.password_reset`
- `whitelist.add`, `whitelist.update`, `whitelist.delete`, `whitelist.bulk_import`
- `blacklist.add`, etc.
- `license.create`, `license.update`, `license.delete`
- `machine.delete`, `machine.tag_update`, `machine.trigger_scan`
- `vuln.dismiss`, `vuln.undismiss`
- `alert.acknowledge`, `alert.resolve`
- `report.generate`, `schedule.create`, `schedule.delete`
- `auth.login_success`, `auth.login_failure`, `auth.logout`, `auth.password_change`
- `policy.update_mode`
- `settings.update_brand`

**Changes JSONB:**
```json
{
  "before": { "purchased_count": 20 },
  "after":  { "purchased_count": 25 }
}
```

For creates: `before` = null
For deletes: `after` = null

---

## UI components

| Component | Notes |
|-----------|-------|
| `LoginForm` | with rate limit feedback |
| `UserTable` | with role badge |
| `UserFormDialog` | create/edit |
| `RoleSelect` | admin/viewer |
| `SessionList` | with revoke buttons |
| `AuditLogTable` | with JSON diff viewer |
| `JSONDiffView` | side-by-side or unified |
| `PasswordStrengthMeter` | live feedback as user types |

---

## Edge cases

1. **Last admin cannot demote self**: backend checks `WHERE role='admin' AND deleted_at IS NULL COUNT > 1` before demote
2. **User soft-deleted but referenced in audit log**: keep audit row; frontend shows "(deleted user)"
3. **Concurrent password change**: optimistic locking by `updated_at` — if mismatch, 409 Conflict
4. **User disabled while logged in**: next request → JWT still valid until expiry, but middleware checks `is_active=true` → 401
5. **Audit log table grows huge**: partition by month + archive > 1 year
6. **Email change** (future): require admin action + re-verify

---

## Test plan

### Unit
- Password policy validator (each rule)
- Rate limit logic
- Common password check

### Integration
- Login happy + sad paths
- 5 wrong attempts → lockout
- Reset password full flow
- Admin create user → new user logs in
- Viewer hits mutation → 403
- Audit log written for each action

### Frontend
- Login form validation
- User table CRUD modal
- Audit log filter combinations
- Session revoke updates list

### Security
- JWT can't be tampered (signature check)
- Refresh token revoke → access fails next refresh
- Password not logged anywhere (grep test)
