# Child Connect

A secure helper for the legal child adoption process in India (CARA / CARINGS). AI helps with paperwork,
information and deadlines; a human always decides. Full specification: `docs-src/allaboutourproject (1) (1).md`.

| Part | Folder | Stack |
|---|---|---|
| Website | `frontend/` | Next.js 16, TypeScript, Tailwind, TanStack Query, next-intl (en / hi / mr) |
| API | `backend/` | Go, chi, pgx, goose, JWT cookies, Gemini (optional), scikit-learn model exported to JSON |
| Database | Neon | PostgreSQL 18 + pgvector |

## Start (Windows PowerShell)

```powershell
.\scripts\start-backend.ps1      # API on http://localhost:8080/api/v1
.\scripts\start-frontend.ps1     # website on http://localhost:3000 (production build)
.\scripts\reset-data.ps1         # restore the clean starting data (stop the API first)
```

## Accounts (all synthetic; password `ChildConnect@2026`)

| Role | Email | What it shows |
|---|---|---|
| Admin | admin@childconnect.in | Deadline Radar, scorecard, child alerts, agency approval, knowledge base, scam reports, audit chain |
| Agency (social worker) | worker@childconnect.in | Asha Shishu Griha, Thane: cases by risk, document review, Report Co-Pilot, children, referrals, legal steps |
| Agency (medical officer) | doctor@childconnect.in | Same agency: medical summary approval |
| Agency (not yet approved) | nagpur@childconnect.in | Blocked until the admin approves Navjeevan Bal Sadan |
| Parent | parent@childconnect.in | Registered, 12 days left; upload the remaining documents from `sample-documents/` |
| Parent | parent2@childconnect.in | Waiting list, highest seniority: receives the referral of child MH-THN-2026-0002 |
| Parent | parent3@childconnect.in | Documents submitted (rent agreement warning) for agency review |
| Parent | parent4@childconnect.in | Home study in progress: Report Co-Pilot (`sample-documents/hsr_rough_notes.txt`) |
| Parent | parent5@childconnect.in | Adopted: follow-up visits (`sample-documents/followup_visit_notes.txt`) |

## AI

Put a Gemini API key in `backend/.env` (`GEMINI_API_KEY=`) and restart the API to switch from rules-only mode to
Gemini (vision DocCheck, generated Sahayak answers, Report Co-Pilot, medical summary, Scam Shield AI layer).
Without a key every feature still works and is labelled `rules-only`.

## Tests

* `backend/tests/e2e_api.py`: 158 end-to-end API checks (all pass), including 14 security cases.
* `go test ./...` in `backend`, `npm run typecheck && npm run lint && npm test && npm run build` in `frontend`.
* Results: `docs/testing-report.md`.
