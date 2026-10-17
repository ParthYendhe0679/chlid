# API integration matrix

"Wired" = typed function in `frontend/lib/api/*.api.ts` used by a screen with loading/error/success states. "Real-API tested" = exercised against the Go backend by `backend/tests/e2e_api.py` (158 checks) and in the browser. Unit tests cover the client only (error mapping, credentials, no POST retry, GET retry).

| Feature | Endpoint | Module | Screen | Status |
|---|---|---|---|---|
| Eligibility | POST /public/eligibility | public.api | /eligibility | Real-API tested |
| Fee Card | GET /public/fees | public.api | /fees | Real-API tested |
| Verify Agency | GET /public/agencies | public.api | /verify-agency | Real-API tested |
| Scam Shield | POST /public/scam-check, /scam-report | public.api | /scam-shield | Real-API tested |
| Auth | /auth/login, register, logout, me, refresh | auth.api, client | /login, /register, shells | Real-API tested |
| Profile | POST /applications, GET/PATCH /applications/me | applications.api | /parent/profile | Real-API tested |
| Tracker | GET /applications/me/tracker | applications.api | /parent | Real-API tested |
| DocCheck | POST /applications/me/documents | applications.api (XHR) | /parent/documents | Real-API tested |
| Submit docs | POST /applications/me/submit | applications.api | /parent/documents | Real-API tested |
| View doc | GET /documents/:id/url | applications.api | documents, review | Real-API tested |
| Referral | GET /referrals/active, POST /referrals/:id/decision | applications.api | /parent/referral | Real-API tested |
| Follow-ups (parent) | GET /followups/me | applications.api | /parent | Real-API tested |
| Messages | GET/POST /applications/:id/messages | applications.api | /parent/messages, case tab | Real-API tested |
| Sahayak | POST /ai/assistant, /escalate | ai.api | /parent/assistant | Real-API tested |
| Case list | GET /agency/cases | agency.api | /agency | Real-API tested |
| Case detail | GET /agency/cases/:id | agency.api | /agency/cases/[id] | Real-API tested |
| Doc review | PATCH /agency/documents/:id/review, POST verify-all | agency.api | case Documents tab | Real-API tested |
| HSR | POST /ai/hsr/draft, PUT/POST /agency/hsr/... | ai.api, agency.api | case HSR tab | Real-API tested |
| Legal events | POST /agency/cases/:id/events | agency.api | case Legal steps tab | Real-API tested |
| Follow-up report | POST /ai/followup/draft, /agency/followups/:id/sign | ai.api, agency.api | case Follow-ups tab | Real-API tested |
| Children | POST /agency/children, GET /agency/children (P1), GET /:id (P2) | agency.api | /agency/children | Real-API tested (P1, P2 implemented) |
| Reports upload | POST /agency/children/:id/reports | agency.api | child detail | Real-API tested |
| Medical summary | POST /ai/medical-summary, POST .../summary/approve | ai.api, agency.api | child detail | Real-API tested |
| Send referral | GET next-parent, POST /agency/referrals | agency.api | child detail | Real-API tested |
| Radar | GET /admin/radar, POST recompute | admin.api | /admin | Real-API tested |
| Scorecard | GET /admin/agency-scorecard | admin.api | /admin | Real-API tested |
| Child alerts | GET /admin/child-alerts | admin.api | /admin/child-alerts | Real-API tested |
| Agency approval | GET/PATCH /admin/agencies | admin.api | /admin/agencies | Real-API tested |
| Knowledge base | POST /admin/knowledge | admin.api | /admin/knowledge | Real-API tested (list added) |
| Scam reports | GET /admin/scam-reports | admin.api | /admin/scams | Real-API tested (mark reviewed added) |
| Audit | GET /admin/audit, /audit/verify | admin.api | /admin/audit | Real-API tested |
| Notifications | GET /notifications, POST /notifications/:id/read, /read-all | notifications.api | bell in every signed-in header | Real-API tested |
| Password recovery | none | - | - | Not built (P8 needs an email service) |
