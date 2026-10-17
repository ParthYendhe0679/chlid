# Frontend / backend contract

Source of truth: `docs-src/allaboutourproject (1) (1).md` section 15. The frontend calls every endpoint below exactly as written there. Base URL: `NEXT_PUBLIC_API_URL`. Requests use `credentials: 'include'` (cookie `cc_session`, httpOnly). Errors follow `{ error: { code, message, request_id } }`.

## Behaviour the frontend relies on

| Topic | Frontend behaviour |
|---|---|
| 401 | Calls `POST /auth/refresh` once, retries the request once, then shows "session expired" and goes to `/login`. |
| Retries | Only `GET` retries once on network failure. POST/PUT/PATCH (uploads, decisions, messages, referrals) are never auto-retried. |
| Uploads | `multipart/form-data`; DocCheck is a single synchronous request (guide 15.4). No stage/progress events exist, so the UI shows real upload progress and then an indeterminate "server is checking" state with measured elapsed time. |
| CSRF | The guide specifies SameSite=Lax only. If the backend adds a CSRF header, the change belongs in `lib/api/client.ts`. |
| `/auth/me` | Envelope not stated in the guide. Frontend accepts `{ user }` or the bare user. |
| 404 on `/applications/me` | Treated as "no application yet". |
| `/referrals/active` | Frontend accepts `null` or 404-free empty body as "none". Please return `null` with 200. |

## Endpoints used (all from the guide)

Public: `POST /public/eligibility`, `GET /public/fees`, `GET /public/agencies`, `POST /public/scam-check`, `POST /public/scam-report`.
Auth: `POST /auth/register`, `/auth/login`, `/auth/refresh`, `/auth/logout`, `GET /auth/me`.
Parent: `POST /applications`, `GET|PATCH /applications/me`, `GET /applications/me/tracker`, `POST /applications/me/documents`, `POST /applications/me/submit`, `GET /documents/:id/url`, `GET /referrals/active`, `POST /referrals/:id/decision`, `GET /followups/me`.
Shared: `GET|POST /applications/:id/messages`, `POST /ai/assistant`, `POST /ai/assistant/escalate`.
Agency: `GET /agency/cases`, `GET /agency/cases/:id`, `PATCH /agency/documents/:id/review`, `POST /agency/cases/:id/verify-all`, `POST /ai/hsr/draft`, `PUT /agency/hsr/:application_id`, `POST /agency/hsr/:application_id/sign`, `POST /agency/children`, `POST /agency/children/:id/reports`, `POST /ai/medical-summary`, `POST /agency/children/:id/summary/approve`, `GET /agency/children/:id/next-parent`, `POST /agency/referrals`, `POST /agency/cases/:id/events`, `POST /ai/followup/draft`, `POST /agency/followups/:id/sign`.
Admin: `GET /admin/radar`, `/admin/agency-scorecard`, `/admin/child-alerts`, `/admin/agencies`, `PATCH /admin/agencies/:id`, `POST /admin/knowledge`, `GET /admin/scam-reports`, `/admin/audit`, `/admin/audit/verify`, `POST /admin/radar/recompute`.

## PROPOSED additions (not in the guide; backend must agree or reject)

| # | Proposal | Why the UI needs it | Frontend behaviour until agreed |
|---|---|---|---|
| P1 | `GET /agency/children` -> `[{id, case_code, status, gender, is_special_needs, age_text}]` | Children list page | Page shows a clear "endpoint missing" notice. |
| P2 | `GET /agency/children/:id` -> list item + `has_csr`, `has_mer`, `summary_ai_draft`, `summary_approved` | Child detail and Referral Lock checklist | Error state shown. |
| P3 | `required_docs[].document_id` and `reviewer_note` on `GET /applications/me` | "View my file" and showing the agency's re-upload reason | Link/note hidden when absent. |
| P4 | `GET /agency/cases/:id` `hsr.ai_run_id` | Signing after a page reload | Signing requires an AI draft in the same session. |
| P5 | `GET /agency/cases/:id` `followups[].id` | Signing follow-ups | Follow-up tab empty. |
| P6 | `level` (`OVERDUE`/`HIGH_RISK`/`ON_TRACK`) on `GET /admin/radar` items; server-side `?level=` filter | Filtering by level | Filter returns whatever the server returns. |
| P7 | `designation` on the `/auth/me` user | Show MEDICAL_OFFICER vs SOCIAL_WORKER | Not used for gating. |
| P8 | `POST /auth/forgot-password`, `/auth/reset-password` | Password recovery (mentioned in the task, absent from the guide) | Not implemented; no page. |
| P9 | `GET /notifications`, `POST /notifications/:id/read` | In-app notifications (guide section 9 lists them, no endpoint) | Not implemented; no bell. |
| P10 | `GET /public/stats` | Data-driven Wait-Time Explainer | Explainer uses only the quoted public figures from guide 3.8. |
| P11 | Address request field name for `POST /applications` | Guide stores `address_enc` but names no request field | Frontend sends `address`. |
| P12 | `GET /admin/knowledge` (list + processing state) | Show which sources are active | Admin only sees the result of the last upload. |
| P13 | Document-job status endpoint (`GET /documents/:id/status`) with stage events | A truthful multi-stage DocCheck terminal | Terminal shows only real events (see client). |
| P14 | Admin scam-report "mark reviewed" (`PATCH /admin/scam-reports/:id`) | The `reviewed` column exists | Not implemented. |

## Open questions for the backend developer

1. Is the 48-hour referral window enforced and returned only by the server? (Frontend only counts down `expires_at`.)
2. Are report URLs (`csr_url`, `mer_url`) already pre-signed with 5-minute expiry? (Frontend offers "Refresh links".)
3. Should Sahayak responses support streaming? (Not in the guide; frontend shows a typing indicator and the full JSON answer.)
4. Does the agency `GET /documents/:id/url` work for agency users of the same agency? (Used in document review.)

## Backend decisions (Go API, October 2026)

Implemented: P1, P2, P3, P4, P5, P6, P7, P9, P10, P11 (`address`), P12, P14. Not implemented: P8 (no email service),
P13 (DocCheck stays one synchronous request, as in guide 15.4).

Answers to the open questions:

1. Yes. The server alone enforces the 48-hour window: decisions after `expires_at` get 409, and the background job
   expires pending referrals (counts as a used referral; the third one debars for 1 year).
2. Yes. `csr_url`, `mer_url` and `GET /documents/:id/url` are HMAC-signed links valid for 5 minutes, created only
   after a permission check, and every request is written to the audit log.
3. No streaming. `POST /ai/assistant` returns the full JSON answer.
4. Yes. Agency users can open documents of applications that belong to their own agency (403 otherwise).

Other behaviour: `/auth/me` returns `{ user }`; `GET /referrals/active` returns `null` with 200 when there is none;
`GET /applications/me` returns 404 before the profile exists; server messages follow the `cc_locale` cookie
(en/hi/mr) and fall back to the user's language.
