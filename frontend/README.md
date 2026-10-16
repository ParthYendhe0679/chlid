# Child Connect frontend

Next.js 16 (App Router), TypeScript strict, Tailwind v4, TanStack Query, React Hook Form + Zod, next-intl (en/hi/mr, `cc_locale` cookie), Recharts, three.js (landing backdrop).

## Run

```bash
cd frontend
cp .env.example .env.local      # set NEXT_PUBLIC_API_URL to the Go API (default http://localhost:8080/api/v1)
npm install
npm run dev                     # http://localhost:3000
npm run typecheck && npm run lint && npm test && npm run build
```

## Demo mode (no backend)

Set `NEXT_PUBLIC_DEMO_MODE=true`. A banner shows on every page and all API calls go to synthetic in-memory data (`lib/demo/handlers.ts`). Accounts: parent@demo.in, parent2@demo.in (has a referral), worker@demo.in, doctor@demo.in, admin@demo.in; password `Demo@1234`. Never enable this in production. It does not run real AI.

## Hero images

Landing and login use `public/hero-1.jpg`, `hero-2.jpg`, `hero-3.jpg` if present (gradient fallback otherwise). Use only images you have rights to, and never real children's photos without consent.

## Docs

`../docs/frontend-backend-contract.md`, `frontend-feature-audit.md`, `api-integration-matrix.md`. Reference documents are in `../docs-src/`.
