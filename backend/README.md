# Child Connect API (Go)

Go 1.22+ server for the Child Connect project guide (`../docs-src/allaboutourproject (1) (1).md`).
Stack as in guide section 11.2: chi, pgx v5 (pgxpool), goose (embedded migrations), golang-jwt v5, bcrypt,
go-playground/validator, adrg/strutil (Jaro-Winkler), google.golang.org/genai (Gemini). Database: Neon PostgreSQL + pgvector.

## Run

```bash
cd backend
cp .env.example .env             # fill DATABASE_URL, JWT_SECRET, FIELD_ENCRYPTION_KEY (and GEMINI_API_KEY for real AI)
go run ./cmd/seed -reset         # wipes the database and loads synthetic data + ../sample-documents
go run ./cmd/api                 # http://localhost:8080/api/v1 (migrations run on start)
python tests/e2e_api.py          # end-to-end API test (changes data; re-run the seed afterwards)
go test ./...
```

Retrain the deadline risk model (writes `internal/ai/risk/model.json`): `python ml/train_risk.py`.

## AI modes

* `GEMINI_API_KEY` set: DocCheck vision, Sahayak answers, Report Co-Pilot, medical summary, Scam Shield AI layer and
  `gemini-embedding-001` (768) embeddings. Knowledge chunks made offline are re-embedded automatically on start.
* No key: every feature still works in **rules-only mode** and says so (`model: rules-only` in `ai_runs` and responses):
  DocCheck reads text PDFs with rules, Sahayak quotes the best matching official passage with its citation
  (local hashed embeddings), the Co-Pilot sorts note sentences into sections without adding facts.

Every AI call: 20 s limit, one retry, output stored in `ai_runs`, human decision recorded (ACCEPTED / EDITED / REJECTED).

## Layout (guide section 18.1)

```
cmd/api        server: config, DB pool, migrations, router, background job (15 min)
cmd/seed       synthetic agencies, staff, 41 cases across all stages, children, knowledge base, sample documents
internal/      config, db, httpx (errors/JSON), auth (JWT cookies, bcrypt, rate limits), audit (hash chain),
               crypto (AES-256-GCM), storage (private files + 5-minute signed links), rules (eligibility, fees,
               documents, en/hi/mr messages), workflow (status rules, deadlines, tracker), ai (Gemini, DocCheck,
               Sahayak, Co-Pilot, Scam Shield), ai/risk (logistic regression scoring), handlers, jobs, samples
migrations/    0001_init.sql (guide section 13 + small marked additions)
ml/            generate_synthetic.py, train_risk.py (scikit-learn, synthetic data only)
tests/         e2e_api.py
```

## Additions to the guide (agreed with the frontend contract)

| Proposal | Implemented as |
|---|---|
| P1/P2 | `GET /agency/children`, `GET /agency/children/:id` (never returns the child's name) |
| P3 | `required_docs[].document_id`, `reviewer_note` on `GET /applications/me` |
| P4/P5 | `hsr.ai_run_id`, `followups[].id` on `GET /agency/cases/:id` |
| P6 | `level` on radar items, `?level=` filter |
| P7 | `designation` (and `agency_name`, `email`) on `/auth/me` |
| P9 | `GET /notifications`, `POST /notifications/:id/read`, `POST /notifications/read-all` |
| P10 | `GET /public/stats` (public figures from guide 3.8) |
| P11 | request field `address`, stored encrypted in `address_enc` |
| P12 | `GET /admin/knowledge` |
| P14 | `PATCH /admin/scam-reports/:id` `{ "reviewed": true }` |
| extra | `GET /admin/model` (risk model card, AI run statistics), `GET /health` |

Schema additions (marked `ADDED` in the migration): `agencies.reviewed_at`, `users.token_version` (logout revokes
refresh tokens), `knowledge_chunks.embed_model`, a few indexes, and a trigger that makes `audit_logs` append-only
(Neon has one owner role, so `REVOKE UPDATE, DELETE` cannot be used).

Not built: password recovery (P8, needs an email service), TOTP two-factor (priority 2), R2 storage (local folder
is used, as the guide allows for the demo).
