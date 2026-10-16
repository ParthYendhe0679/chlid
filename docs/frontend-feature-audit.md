# Frontend feature audit

Repository state at start: only the three reference documents existed (no code). Everything below was built new in `frontend/`. Reference docs moved to `docs-src/`.

Status legend: DONE (built, builds, type-checks; verified only as noted), DEMO (works only in demo mode against synthetic data because the real backend is not available to me), BLOCKED (needs backend/product decision).

"Verified" means: TypeScript, lint, production build and 12 unit tests pass. In a browser I verified only: home page render and demo-mode parent login -> Case Tracker. Nothing was tested against the real Go API, and no end-to-end (Playwright) suite exists.

| Area | Requirement (guide section) | Status | Notes |
|---|---|---|---|
| Design | DESIGN.md tokens, clay/sticker components, pill nav, mobile dock, sheets, toasts, skeletons, reduced motion | DONE | Tailwind v4 tokens in `app/globals.css`. Icons are lucide-react (task prompt) instead of Phosphor (DESIGN.md); no `motion` library, so the nav pill has no spring animation. |
| Design conflict | Guide 17.3 asks for "deep blue, calm, government look"; DESIGN.md is playful clay/sticker | DONE | DESIGN.md followed per the stated precedence. Guide rules kept: no emojis, AI label on every AI output, days-left with legal rule on hover, mobile-first. |
| Public | Home, guidance, Eligibility, Fee Card, Verify Agency, Scam Shield | DONE | Fee values come only from the API. |
| Public | Password recovery | BLOCKED | No endpoint in guide (P8). |
| Auth | Login, register with consent, logout, session restore, expired session, role guards, access-denied | DONE | UX guards only. `proxy.ts` not used because the session cookie lives on the API origin. |
| Parent | Tracker, profile + preferences, Wait-Time Explainer | DONE | Explainer is qualitative (no stats endpoint, P10). |
| Parent | Documents + DocCheck terminal, submit with warnings | DONE | Terminal logs only real signals (see contract). No fabricated percentages or stages. |
| Parent | Referral + 48h timer + reserve/decline + debarment warnings | DONE | Server result shown after decision. |
| Parent | Messages | DONE | Refresh on focus and manually; no polling. |
| Parent | Sahayak AI incl. voice | DONE | Citations, copy, listen (speechSynthesis), mic via browser SpeechRecognition with disclosure and transcript preview. Not streaming (backend returns JSON). Voice depends on browser support; Marathi voices are often missing and the UI says so. Conversation is never persisted. |
| Parent | Follow-up schedule | DONE | Read-only. |
| Parent | Approved medical summary + reports | DONE | Shown inside referral page. |
| Agency | Case list by risk, case detail tabs | DONE | |
| Agency | Document review (split view, verify, re-upload request, verify-all) | DONE | Preview via 5-minute URL in sandboxed iframe/img. Needs P3. |
| Agency | Report Co-Pilot (HSR) draft/edit/save/sign | DONE | Distinguishes AI draft from final; dirty tracking; never overwrites edits silently. Signing needs `ai_run_id` (P4). |
| Agency | Follow-up report draft/sign | DONE | Needs P5. |
| Agency | Legal events | DONE | Server enforces transitions (409 shown). |
| Agency | Children, reports upload, medical summary approve, Referral Lock checklist, send referral | DONE | Needs P1, P2. Child name is sent once, never displayed. |
| Admin | Deadline Radar, filters, search, scorecard chart, recompute | DONE | Predictions labelled; missing score shown as "no prediction available". Pagination not implemented (guide has no paging params). |
| Admin | Child alerts, agency approval, knowledge base, scam reports, audit + verify chain | DONE | KB list and "mark reviewed" missing (P12, P14). Admin cannot edit case content. |
| i18n | en / hi / mr | PARTIAL | English complete. Hindi and Marathi cover navigation, auth, home, eligibility, fees, tracker, documents, assistant, referral basics and statuses; remaining keys fall back to English. Machine-written translations need a native-speaker review. Language stored in a `cc_locale` cookie (no `[locale]` route segment, a deviation from guide 17.2 to keep URLs as in 17.1). |
| ML | Prediction display | DONE | Model itself is backend work. |
| Notifications | Bell / list | BLOCKED | P9. |
| Tests | Unit/API/validation/i18n-key tests | PARTIAL | 12 tests. No component, workflow or Playwright E2E tests yet. |
| A11y | Labels, focus ring, aria-live, tabs keyboard, dialog focus trap | PARTIAL | Built in; no automated or screen-reader audit done. |
| Demo mode | Isolated synthetic data, banner | DONE | `NEXT_PUBLIC_DEMO_MODE=true`; only a subset of agency/admin handlers are realistic. |
