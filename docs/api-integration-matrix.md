# API integration matrix

"Wired" = typed function in `frontend/lib/api/*.api.ts` used by a screen with loading/error/success states. "Real-API tested" = none yet; the Go backend was not available. Unit tests cover the client only (error mapping, credentials, no POST retry, GET retry).

| Feature | Endpoint | Module | Screen | Status |
|---|---|---|---|---|
| Eligibility | POST /public/eligibility | public.api | /eligibility | Wired |
| Fee Card | GET /public/fees | public.api | /fees | Wired |
| Verify Agency | GET /public/agencies | public.api | /verify-agency | Wired |
| Scam Shield | POST /public/scam-check, /scam-report | public.api | /scam-shield | Wired |
| Auth | /auth/login, register, logout, me, refresh | auth.api, client | /login, /register, shells | Wired |
| Profile | POST /applications, GET/PATCH /applications/me | applications.api | /parent/profile | Wired (P11) |
| Tracker | GET /applications/me/tracker | applications.api | /parent | Wired |
| DocCheck | POST /applications/me/documents | applications.api (XHR) | /parent/documents | Wired |
| Submit docs | POST /applications/me/submit | applications.api | /parent/documents | Wired |
| View doc | GET /documents/:id/url | applications.api | documents, review | Wired (P3) |
| Referral | GET /referrals/active, POST /referrals/:id/decision | applications.api | /parent/referral | Wired |
| Follow-ups (parent) | GET /followups/me | applications.api | /parent | Wired |
| Messages | GET/POST /applications/:id/messages | applications.api | /parent/messages, case tab | Wired |
| Sahayak | POST /ai/assistant, /escalate | ai.api | /parent/assistant | Wired |
| Case list | GET /agency/cases | agency.api | /agency | Wired |
| Case detail | GET /agency/cases/:id | agency.api | /agency/cases/[id] | Wired (P4, P5) |
| Doc review | PATCH /agency/documents/:id/review, POST verify-all | agency.api | case Documents tab | Wired |
| HSR | POST /ai/hsr/draft, PUT/POST /agency/hsr/... | ai.api, agency.api | case HSR tab | Wired |
| Legal events | POST /agency/cases/:id/events | agency.api | case Legal steps tab | Wired |
| Follow-up report | POST /ai/followup/draft, /agency/followups/:id/sign | ai.api, agency.api | case Follow-ups tab | Wired |
| Children | POST /agency/children, GET /agency/children (P1), GET /:id (P2) | agency.api | /agency/children | Wired; list/detail need proposals |
| Reports upload | POST /agency/children/:id/reports | agency.api | child detail | Wired |
| Medical summary | POST /ai/medical-summary, POST .../summary/approve | ai.api, agency.api | child detail | Wired |
| Send referral | GET next-parent, POST /agency/referrals | agency.api | child detail | Wired |
| Radar | GET /admin/radar, POST recompute | admin.api | /admin | Wired (P6) |
| Scorecard | GET /admin/agency-scorecard | admin.api | /admin | Wired |
| Child alerts | GET /admin/child-alerts | admin.api | /admin/child-alerts | Wired |
| Agency approval | GET/PATCH /admin/agencies | admin.api | /admin/agencies | Wired |
| Knowledge base | POST /admin/knowledge | admin.api | /admin/knowledge | Wired (P12) |
| Scam reports | GET /admin/scam-reports | admin.api | /admin/scams | Wired (P14) |
| Audit | GET /admin/audit, /audit/verify | admin.api | /admin/audit | Wired |
| Notifications | none | - | - | Blocked (P9) |
| Password recovery | none | - | - | Blocked (P8) |
