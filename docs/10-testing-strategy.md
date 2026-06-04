# 10 — Testing Strategy

## Test Pyramid

```
            ▲
           /e2e\               ~5%  — Playwright (Dashboard → Backend → DB)
          /─────\
         /  int. \             ~20% — Integration (Backend + DB, Agent + mock server)
        /─────────\
       /   unit    \           ~75% — Pure functions, services, components
      /─────────────\
```

หลักการ: **เร็วและเยอะข้างล่าง, ช้าและน้อยข้างบน**

---

## Backend (Python)

### Unit tests
**Scope:** services, parsers, validators — ไม่แตะ DB จริง

**Location:** `backend/tests/services/`, `backend/tests/core/`

```python
# tests/services/test_cve_matcher.py
import pytest
from app.services.cve_matcher import is_version_affected

@pytest.mark.parametrize("installed,range_spec,expected", [
    ("1.0.0", ">=1.0,<2.0", True),
    ("2.0.0", ">=1.0,<2.0", False),
    ("0.9.9", ">=1.0,<2.0", False),
])
def test_version_range(installed, range_spec, expected):
    assert is_version_affected(installed, range_spec) is expected
```

### Integration tests
**Scope:** API endpoints → real Postgres (testcontainers or shared test DB)

**Location:** `backend/tests/api/`

**Pattern**: pytest fixture สำหรับ DB + HTTP client
```python
# conftest.py
@pytest.fixture
async def db_session():
    # transactional rollback — ทุก test isolated
    async with engine.begin() as conn:
        nested = await conn.begin_nested()
        session = AsyncSession(bind=conn, ...)
        yield session
        await nested.rollback()

@pytest.fixture
async def client(db_session):
    app.dependency_overrides[get_db] = lambda: db_session
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as c:
        yield c
    app.dependency_overrides.clear()

@pytest.fixture
async def admin_token(client):
    # seed admin + login
    ...

# tests/api/test_machines.py
async def test_list_machines_requires_auth(client):
    res = await client.get("/api/v1/machines")
    assert res.status_code == 401

async def test_list_machines_filters_by_status(client, admin_token, seed_machines):
    res = await client.get("/api/v1/machines?status=online", headers={"Authorization": f"Bearer {admin_token}"})
    assert res.status_code == 200
    assert all(m["status"] == "online" for m in res.json()["items"])
```

### Mock external services
- NVD/OSV API → `respx` (httpx mock) หรือ pytest fixture
- Never call real external in tests

### Coverage target
- Services: 90%+
- API: 80%+ (happy path + error path of each endpoint)
- Models/schemas: 60% (tests via API ก็พอ)

---

## Agent (Go)

### Unit tests
**Scope:** parsers (registry, plist), version compare, signature mapping, queue

**Location:** `agent/internal/*/`, file `*_test.go` ข้างไฟล์เดิม

```go
func TestParseUninstallEntry(t *testing.T) {
    tests := []struct {
        name string
        key  map[string]string
        want Software
    }{
        {
            name: "valid entry",
            key:  map[string]string{"DisplayName": "Chrome", "DisplayVersion": "1.0"},
            want: Software{Name: "Chrome", Version: "1.0"},
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := parseUninstallEntry(tt.key)
            if got != tt.want { t.Errorf("got %+v want %+v", got, tt.want) }
        })
    }
}
```

### Build tag tests
```go
//go:build windows

func TestRegistryReader(t *testing.T) { ... }
```
Run: `go test -tags=windows ./...` (CI matrix)

### Integration tests
**Scope:** Agent + httptest server simulating backend

**Location:** `agent/test/integration/`

```go
func TestEnrollmentFlow(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // assert request shape, return mock response
    }))
    defer srv.Close()

    cfg := config.Default()
    cfg.ServerURL = srv.URL
    agent := New(cfg)
    err := agent.Enroll("test-token")
    require.NoError(t, err)
}
```

### Coverage target
- Pure logic: 85%+
- OS-specific (Windows/macOS): 60% (limited by CI runner OS)

---

## Dashboard (TypeScript)

### Unit tests (Vitest)
**Scope:** utility functions, pure components, hooks

**Location:** `dashboard/{components,lib,hooks}/**/*.test.{ts,tsx}`

```tsx
// components/MachineRow.test.tsx
import { render, screen } from '@testing-library/react'
import { MachineRow } from './MachineRow'

test('renders hostname and status', () => {
  render(<MachineRow machine={{ uuid: '1', hostname: 'pc-01', status: 'online', ... }} />)
  expect(screen.getByText('pc-01')).toBeInTheDocument()
  expect(screen.getByLabelText(/online/i)).toBeInTheDocument()
})
```

### Component tests (React Testing Library)
- Render + assert by **role** หรือ **label**, ไม่ใช่ by class/id
- Mock fetch ผ่าน MSW (`msw` library)

### E2E tests (Playwright)
**Scope:** Golden paths through real stack

**Location:** `dashboard/e2e/`

```ts
// e2e/login-and-list.spec.ts
test('admin can login and see machines list', async ({ page }) => {
  await page.goto('/login')
  await page.fill('[name=email]', 'admin@local')
  await page.fill('[name=password]', 'ChangeMe!2026')
  await page.click('button[type=submit]')
  await expect(page).toHaveURL('/')
  await page.click('text=Machines')
  await expect(page.locator('h1')).toHaveText(/Machines/)
})
```

**Run:** `pnpm test:e2e` (starts docker compose, runs tests, tears down)

### Coverage target
- Vitest line coverage: 70%+
- E2E: cover 1 golden path per page minimum

---

## Cross-stack integration tests

**Scope:** Full stack via Docker Compose

**Location:** root `tests/integration/`

**Approach:**
1. `docker compose up` (test profile)
2. Wait for health endpoints
3. Run pytest scenarios:
   - Agent simulator (Python script) enrolls → POST scan → backend stores → assert via API
   - Bulk scenario: 100 machines, 50 software each → assert dashboard query returns correct aggregate
4. `docker compose down -v`

Run on CI nightly, not on every PR (slow)

---

## CI matrix

GitHub Actions `.github/workflows/ci.yml`:

```yaml
jobs:
  backend:
    runs-on: ubuntu-latest
    services:
      postgres: ...
      redis: ...
    steps:
      - uv sync
      - uv run ruff check .
      - uv run mypy app
      - uv run pytest --cov

  agent:
    strategy:
      matrix:
        os: [ubuntu-latest, windows-latest, macos-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - go test ./...
      - golangci-lint run

  dashboard:
    runs-on: ubuntu-latest
    steps:
      - pnpm install
      - pnpm lint && pnpm typecheck
      - pnpm test
      - pnpm build

  e2e:
    runs-on: ubuntu-latest
    needs: [backend, dashboard]
    steps:
      - docker compose -f docker-compose.test.yml up -d
      - pnpm test:e2e
```

---

## When to write tests

| Code change | Test required? |
|-------------|----------------|
| New API endpoint | ✅ integration test (happy + auth + validation) |
| New service function (business logic) | ✅ unit test |
| New model field | ⚠️ Migration test (apply + rollback) |
| New UI component (logic-heavy) | ✅ unit/component test |
| New UI page | ✅ e2e golden path |
| Pure styling change | ❌ |
| Refactor (no behavior change) | ❌ existing tests must still pass |
| Bug fix | ✅ regression test that fails before, passes after |

---

## TDD discipline

**For business logic** (CVE matching, license calculation, risk scoring):
1. เขียน test ที่ fail ก่อน
2. เขียน implementation ขั้นต่ำที่ pass
3. Refactor

**สำหรับ glue code** (router, ORM model): test ทีหลังก็ได้ แต่ต้องมี

**ห้าม merge** code production path ที่ไม่มี test

---

## Test data

- ใช้ factory pattern: `factory-boy` (Python) / `@faker-js/faker` (TS)
- ห้าม share state between tests
- ห้าม dependency on test order
- Snapshot tests ใช้ได้สำหรับ stable UI, ไม่ใช้กับ business output

---

## Performance tests (Phase 5)

- `k6` script ใน `tests/perf/`
- Scenarios: 1000 agents heartbeat, dashboard list 10k machines
- Target: p95 < 500ms ทุก endpoint

---

## Test environment hygiene

- ไม่ใช้ production credentials ใน test
- Test DB ใช้ schema เดียวกับ prod แต่ port/host แยก
- ทุก test รัน clean → ไม่ leave residue
