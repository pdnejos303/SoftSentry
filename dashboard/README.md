# SoftSentry Dashboard

Next.js 14 (App Router) • TypeScript strict • Tailwind • shadcn/ui • next-intl (th/en) • TanStack Query

## Quick start

```bash
pnpm install
pnpm dev          # http://localhost:3000
```

Set `NEXT_PUBLIC_API_URL=http://localhost:8000/api/v1` in `.env.local` (or rely on root `.env`)

## Commands

```bash
pnpm lint
pnpm typecheck
pnpm test          # vitest unit
pnpm test:e2e      # playwright (Phase 5)
pnpm build && pnpm start
```

## Structure

```
app/[locale]/        — locale-prefixed routes (th, en)
  layout.tsx         — RootLayout (next-intl provider)
  page.tsx           — Marketing / overview placeholder
  login/page.tsx     — Sign-in form
components/ui/       — shadcn primitives (Button, Input, Card, Label)
i18n/                — next-intl routing + request config
lib/
  api.ts             — axios instance + refresh interceptor
  auth.tsx           — AuthProvider + useAuth hook
  query-client.tsx   — TanStack Query provider
  utils.ts           — cn() helper
messages/            — th.json, en.json
```

Convention และ Module spec ดูใน `docs/modules/07-dashboard.md`
