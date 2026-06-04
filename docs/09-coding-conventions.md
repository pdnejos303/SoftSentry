# 09 — Coding Conventions

## Universal rules (ทุกภาษา)

- **อ่าน spec ก่อนเขียน code**
- **เขียน test ก่อนหรือพร้อม implementation** สำหรับ business logic
- **Naming**: เขียน function/variable ให้บอก intent. ไม่ใช้ shorthand เช่น `mgr`, `svc` ถ้าไม่จำเป็น
- **Functions เล็ก** — ถ้าเกิน 50 บรรทัด หรือ depth > 3 → refactor
- **ห้าม commented-out code** — ลบหรือใช้ git history
- **ไม่ใช้ magic numbers** — ตั้ง constant พร้อมชื่อ
- **ห้าม TODO/FIXME ค้างใน main** — เปิด issue หรือทำให้เสร็จ
- **Error message ภาษาอังกฤษ** ใน code (สำหรับ log), Thai/EN ใน UI (ผ่าน i18n)
- **Commit message**: imperative mood (`Add scan endpoint`, ไม่ใช่ `Added`) — Conventional Commits ไม่บังคับ

---

## Go (Agent)

### Tools
- `gofmt` (auto-format)
- `golangci-lint` (lint) — config ใน `agent/.golangci.yml`
- `go test ./...` + `go vet`

### Style
- Standard Go conventions ([Effective Go](https://go.dev/doc/effective_go))
- Package names: short, lowercase, no underscore
- Exported: `PascalCase`, unexported: `camelCase`
- Errors:
  ```go
  if err != nil {
      return fmt.Errorf("scan registry %s: %w", key, err)
  }
  ```
- ห้าม panic ใน production path. Library boundary recover ได้:
  ```go
  defer func() {
      if r := recover(); r != nil {
          log.Error("panic in scan", "err", r, "stack", debug.Stack())
      }
  }()
  ```
- Context: pass `context.Context` เป็นพารามแรกของ I/O / long ops
- Interface: define on consumer side, not producer

### Project layout
```
agent/
├── cmd/softsentry-agent/main.go       # entrypoint
├── internal/
│   ├── config/                        # viper config
│   ├── scanner/                       # software inventory
│   │   ├── scanner.go                 # interface + dispatch by OS
│   │   ├── windows_registry.go        # //go:build windows
│   │   └── macos_apps.go              # //go:build darwin
│   ├── signature/                     # verify
│   │   ├── windows_authenticode.go    # //go:build windows
│   │   └── macos_codesign.go          # //go:build darwin
│   ├── transport/                     # HTTP client
│   ├── queue/                         # local file-based retry queue
│   └── service/                       # OS service wrapper
├── pkg/                               # exported (none for v1)
└── go.mod
```

### Logging
- `log/slog` (stdlib)
- Structured: `slog.Info("scan complete", "duration_ms", d, "software_count", n)`
- Levels: `Debug` (verbose), `Info` (normal), `Warn` (recoverable), `Error` (action needed)

### Concurrency
- `sync.Mutex` for shared state
- Channels สำหรับ pipeline / fan-out
- `errgroup` สำหรับ parallel ops ที่อาจ error
- ห้าม leak goroutine — ทุก goroutine ต้องมี exit condition + context cancel

### Testing
- `testing` stdlib + `testify/assert` ถ้าจำเป็น
- Table-driven tests
- File: `*_test.go` ข้างไฟล์ที่ test
- Mock OS-specific via interface + build tags
- Integration test: `//go:build integration` + `go test -tags=integration`

---

## Python (Backend)

### Tools
- `ruff` (lint + format) — config ใน `backend/pyproject.toml`
- `black` (format — ruff format ใช้แทนได้)
- `mypy` (strict) — config ใน `pyproject.toml`
- `pytest` + `pytest-asyncio`

### Style
- PEP 8 + PEP 257 (docstrings)
- Type hints บังคับ ทุก function signature
- `async def` สำหรับ I/O (DB, HTTP, file)
- Pydantic v2: `BaseModel` สำหรับ schema, `ConfigDict(frozen=True)` ถ้าควร immutable
- Imports: stdlib → third-party → local, sorted (ruff auto)
- F-string > `.format()` > `%`
- ห้าม `from x import *`
- ห้าม mutable default argument (`def f(x: list = []) ...`)

### pyproject.toml essentials
```toml
[tool.ruff]
line-length = 100
target-version = "py312"
[tool.ruff.lint]
select = ["E", "F", "W", "I", "N", "UP", "B", "A", "C4", "T20", "SIM", "RUF"]
ignore = ["E501"]   # line length handled by formatter

[tool.mypy]
strict = true
python_version = "3.12"

[tool.pytest.ini_options]
asyncio_mode = "auto"
addopts = "-ra --cov=app --cov-report=term-missing"
```

### Project layout
```
backend/
├── app/
│   ├── main.py                  # FastAPI app + lifespan
│   ├── core/
│   │   ├── config.py            # pydantic-settings Settings
│   │   ├── db.py                # AsyncSession factory
│   │   ├── security.py          # JWT + bcrypt
│   │   └── deps.py              # FastAPI dependencies (get_db, get_current_user)
│   ├── models/                  # SQLAlchemy ORM
│   │   ├── base.py              # DeclarativeBase + mixins
│   │   ├── user.py
│   │   ├── machine.py
│   │   ├── software.py
│   │   └── ...
│   ├── schemas/                 # Pydantic
│   │   ├── user.py
│   │   ├── machine.py
│   │   └── ...
│   ├── api/v1/                  # Routers
│   │   ├── __init__.py          # api_router include all
│   │   ├── auth.py
│   │   ├── agents.py
│   │   ├── machines.py
│   │   └── ...
│   ├── services/                # Business logic (pure-ish)
│   │   ├── auth.py
│   │   ├── enrollment.py
│   │   ├── cve_matcher.py
│   │   └── license_calculator.py
│   └── workers/
│       ├── settings.py          # arq WorkerSettings
│       ├── cve_sync.py
│       └── alert_engine.py
├── alembic/
│   ├── env.py
│   └── versions/
├── tests/
│   ├── conftest.py              # fixtures: db, client, auth_token
│   ├── api/
│   └── services/
└── pyproject.toml
```

### Router pattern
```python
# app/api/v1/machines.py
from fastapi import APIRouter, Depends, Query
from app.core.deps import get_db, require_role
from app.schemas.machine import MachineRead, MachineListResponse
from app.services.machines import list_machines

router = APIRouter(prefix="/machines", tags=["machines"])

@router.get("", response_model=MachineListResponse)
async def list_(
    q: str | None = Query(None),
    status: str | None = Query(None),
    page: int = Query(1, ge=1),
    page_size: int = Query(50, ge=1, le=200),
    db = Depends(get_db),
    _ = Depends(require_role("admin", "viewer")),
) -> MachineListResponse:
    return await list_machines(db, q=q, status=status, page=page, page_size=page_size)
```

### SQLAlchemy 2 patterns
```python
# Always async
async def get_machine_by_uuid(db: AsyncSession, uuid: UUID) -> Machine | None:
    result = await db.execute(select(Machine).where(Machine.uuid == uuid))
    return result.scalar_one_or_none()

# Use select() not Query (legacy)
# Use scalars().all() for ORM rows
```

### Error handling
```python
# Domain errors → custom exceptions
class MachineNotFound(Exception): ...

# FastAPI handler maps to HTTP
@app.exception_handler(MachineNotFound)
async def handle_machine_not_found(req, exc):
    return JSONResponse(status_code=404, content={"error": {"code": "MACHINE_NOT_FOUND", "message": str(exc)}})
```

---

## TypeScript (Dashboard)

### Tools
- `eslint` (with `eslint-config-next`)
- `prettier`
- `typescript --strict`

### tsconfig.json essentials
```json
{
  "compilerOptions": {
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "noImplicitOverride": true,
    "moduleResolution": "bundler",
    "target": "es2022"
  }
}
```

### Style
- ห้าม `any` — ใช้ `unknown` ถ้าจำเป็น
- ห้าม `as` cast เว้นแต่จะ narrow ผ่าน type guard
- Prefer `type` over `interface` ยกเว้น public API ที่ extend ได้
- Component naming: PascalCase, file = component
- Hooks: `useFoo`, prefix `use`
- Tailwind class: ตรงไหนซ้ำ > 3 ครั้ง → extract เป็น component
- ห้าม magic class string — ใช้ `cn()` util (clsx)

### Project layout
```
dashboard/
├── app/                              # App Router
│   ├── (auth)/login/page.tsx
│   ├── (dashboard)/
│   │   ├── layout.tsx                # auth-protected layout
│   │   ├── page.tsx                  # overview
│   │   ├── machines/
│   │   │   ├── page.tsx              # list
│   │   │   └── [uuid]/page.tsx       # detail
│   │   ├── software/
│   │   ├── vulnerabilities/
│   │   ├── licenses/
│   │   ├── alerts/
│   │   ├── reports/
│   │   └── settings/
│   ├── api/                          # Route handlers (proxy ถ้าจำเป็น)
│   ├── layout.tsx
│   └── globals.css
├── components/
│   ├── ui/                           # shadcn primitives (auto-generated)
│   ├── machines/                     # domain components
│   ├── shared/                       # cross-domain
│   └── layout/
├── lib/
│   ├── api.ts                        # fetch wrapper
│   ├── auth.ts                       # token mgmt
│   ├── utils.ts                      # cn(), formatters
│   └── types.ts                      # shared types (mirror backend Pydantic)
├── hooks/
├── messages/                         # i18n
│   ├── th.json
│   └── en.json
├── public/
└── package.json
```

### Server Components vs Client
- **Default**: Server Component (no `'use client'`)
- **Client** เมื่อต้อง: `useState`, `useEffect`, event handler, browser API
- Fetch ใน Server Component → return data → pass to Client Component as prop

### React Query
- ทุก fetch ผ่าน `useQuery` / `useMutation` (Client side)
- QueryKey: array, hierarchical: `['machines', 'list', {status, q}]`
- Default `staleTime: 30_000`, `retry: 1`
- Mutation success → `queryClient.invalidateQueries(...)` กล่ม key ที่เกี่ยวข้อง

### Forms
```tsx
const schema = z.object({ email: z.string().email(), password: z.string().min(12) })
type FormValues = z.infer<typeof schema>

const form = useForm<FormValues>({ resolver: zodResolver(schema) })
```

### Accessibility
- Use shadcn (Radix-based) — a11y by default
- All interactive: keyboard navigable
- Alt text for images
- Color contrast WCAG AA

### i18n
- Wrap ทุก string ผ่าน `useTranslations`:
  ```tsx
  const t = useTranslations('Machines')
  return <h1>{t('title')}</h1>
  ```
- ห้าม hardcode Thai/English ใน JSX
- Key naming: `<Namespace>.<key>` เช่น `Machines.title`, `Machines.searchPlaceholder`

---

## Git workflow

- Branch naming: `feat/`, `fix/`, `chore/`, `docs/`, `refactor/`, `test/`
  - เช่น `feat/agent-enrollment`, `fix/heartbeat-race`
- Main branch protected: PR + 1 approval + CI green
- Squash merge to main
- Tags: `v1.0.0` (semver) สำหรับ release

## PR checklist
- [ ] Tests added/updated
- [ ] Linter passes
- [ ] Type check passes
- [ ] Docs updated if API/schema changed
- [ ] Migration reversible (if any)
- [ ] No secrets in diff (grep `JWT_SECRET`, `password=`, etc.)
