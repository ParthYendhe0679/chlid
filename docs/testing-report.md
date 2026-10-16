# Testing report (measured, 17 October 2026)

Only numbers we measured are listed here (guide section 22). AI features were tested in **rules-only mode**
(no Gemini key configured at the time of testing). Results with Gemini must be measured again once the key is set.

## End-to-end API test (`backend/tests/e2e_api.py`)

Result: **158 of 158 checks passed** on a freshly seeded database.

Covers: public tools (eligibility incl. the guide's composite-age example, fees in Hindi, Verify Agency, Scam Shield
and report), registration, consent, cookies (httpOnly, SameSite=Lax), refresh and logout revocation, lockout after 5
failed logins and the login rate limit, profile rules (age gap, composite age, single man and girl child), DocCheck,
document review and re-upload, verify-all, Home Study Report draft / save / sign (cannot sign "needs more info"),
children and the Referral Lock (409 before reports and approved summary), referral 1 of 3 not reserved, referral 2 of 3
reserved, every legal event in order (and 409 when out of order), 4 follow-up reports to COMPLETED, messages,
Sahayak (English, Hindi, Marathi, own-case escalation, off-topic refusal), admin radar, scorecard, child alerts,
agency approval, knowledge upload, scam reports, audit log and hash-chain verification.

## Security (guide 22.6)

14 permission cases tested; **all 14 returned 403** and were written to the audit log as `FORBIDDEN_ATTEMPT`:
parent -> agency list, parent -> admin radar, parent -> another parent's messages, parent -> another parent's
document, agency -> audit log, agency -> agency approval, other agency -> case, child, document review and messages
of a Thane case, admin -> edit a Home Study Report, admin -> agency case list, parent -> another parent's referral
decision, unapproved agency -> agency pages. Tampered or expired file links return 403.

## DocCheck on the sample documents (rules-only mode, Go unit test `internal/samples`)

| Sample | Category | Result |
|---|---|---|
| Electricity bill, 20 days old | RESIDENCE | PASS (type allowed, name match, recent bill, address match) |
| Electricity bill, 5 months old | RESIDENCE | WARN (older than 3 months) |
| Rent agreement | RESIDENCE | WARN (not in the usual list) |
| Aadhaar / PAN / birth certificate / salary slip / marriage certificate / reference letter | matching category | PASS |
| Medical certificate without registration number | MEDICAL | WARN |
| Electricity bill uploaded as income proof | INCOME | FAIL (type not allowed) |
| Blurry photo (Laplacian variance 1.1; sharp photo 683.5; threshold 40) | PHOTO | FAIL |
| Same file in two applications | any | FAIL (duplicate) |
| Name matching (Jaro-Winkler 0.88): 5 same-person variants matched, 3 different people rejected | - | 8 of 8 correct |

Photos and image scans are marked "Needs human review" in rules-only mode; the Gemini vision call reads them.

## Risk model (synthetic data only)

Logistic Regression trained on 5,000 synthetic deadlines (`backend/ml`), test split of 1,250:
accuracy 0.734, precision 0.769, recall 0.802, ROC-AUC 0.799. Gradient Boosting comparison: ROC-AUC 0.791.
These are results on synthetic data, not on real agency history.

## Speed (local machine to Neon, ap-southeast-1)

Average over the e2e run: normal APIs 649 ms (144 calls), AI APIs in rules-only mode 575 ms (32 calls).
Most of the time is network round trips to the database region; deploying the API in the same region reduces it.

## Browser walkthrough (built-in browser, development build)

Parent: tracker, document upload with the DocCheck terminal (rent agreement warning, bill pass), submit, Sahayak.
Agency: case list by risk, document review, Report Co-Pilot draft -> edit -> save -> sign, child medical summary ->
approve -> Referral Lock opens -> next parent -> send referral. Parent: referral with 48-hour timer -> reserve.
Admin: radar, child alerts, agency approval, knowledge sources, audit chain verify. Public pages in Hindi.

Bugs found and fixed during testing: dates masked as ID numbers; doctor registration number read from non-medical
certificates; a waiting family's expired home study hid them from the queue (seed); too many "no family visit"
alerts (seed); weak "staff count" risk reason; missing-items box not updating after edits; generic error text hid
the server's explanation; Neon pooler statement cache after a schema reset.
