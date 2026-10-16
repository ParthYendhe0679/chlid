# Child Connect frontend

Next.js 16 (App Router), TypeScript strict, Tailwind v4, TanStack Query, React Hook Form + Zod, next-intl (en/hi/mr, `cc_locale` cookie), Recharts, three.js (landing backdrop).

## Run

```bash
cd frontend
echo NEXT_PUBLIC_API_URL=http://localhost:8080/api/v1 > .env.local
npm install
npm run dev                     # http://localhost:3000
npm run typecheck && npm run lint && npm test && npm run build
```

## Backend

All data comes from the Go API in `../backend` (see its README). There is no demo or mock mode: start the API
first, then `npm run dev` (or `npm run build && npm run start`).

## Hero images

Landing and login use `public/hero-1.jpg`, `hero-2.jpg`, `hero-3.jpg` if present (gradient fallback otherwise). Use only images you have rights to, and never real children's photos without consent.

## Docs

`../docs/frontend-backend-contract.md`, `frontend-feature-audit.md`, `api-integration-matrix.md`. Reference documents are in `../docs-src/`.
