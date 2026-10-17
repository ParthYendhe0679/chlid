-- +goose Up
-- Schema from section 13 of the project guide. Additions are marked "ADDED" and explained.

-- ========== EXTENSIONS ==========
CREATE EXTENSION IF NOT EXISTS pgcrypto;   -- gen_random_uuid(), digest()
CREATE EXTENSION IF NOT EXISTS vector;     -- pgvector

-- ========== ENUM TYPES ==========
CREATE TYPE user_role AS ENUM ('PARENT','AGENCY','ADMIN');

CREATE TYPE app_status AS ENUM (
  'REGISTERED','DOCS_SUBMITTED','DOCS_CORRECTION','HSR_IN_PROGRESS','WAITING','REFERRED','RESERVED',
  'MATCHED_FOSTER_CARE','DM_ORDER_PENDING','ADOPTED_FOLLOWUP','COMPLETED','LAPSED','DEBARRED','WITHDRAWN'
);

CREATE TYPE doc_category AS ENUM (
  'PHOTO','IDENTITY','BIRTH_PROOF','RESIDENCE','INCOME','MEDICAL',
  'MARRIAGE','DIVORCE_OR_DEATH','REFERENCE','CHILD_CONSENT'
);

CREATE TYPE doc_status AS ENUM (
  'AI_PASS','AI_WARN','AI_FAIL','NEEDS_HUMAN','VERIFIED','REUPLOAD_REQUESTED'
);

CREATE TYPE deadline_step AS ENUM (
  'DOC_UPLOAD_30D','HSR_60D','CSR_MER_10D','RESERVE_48H',
  'MATCHING_30D','FOSTER_CARE_10D','DM_FILING_10D','DM_ORDER_60D','FOLLOWUP_VISIT'
);

CREATE TYPE deadline_state AS ENUM ('OPEN','DONE','OVERDUE','CANCELLED');

-- ========== AGENCIES AND USERS ==========
CREATE TABLE agencies (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name          TEXT NOT NULL,
  reg_number    TEXT UNIQUE NOT NULL,
  agency_type   TEXT NOT NULL CHECK (agency_type IN ('SAA','DCPU')),
  state         TEXT NOT NULL,
  district      TEXT NOT NULL,
  staff_count   INT  NOT NULL DEFAULT 1,
  phone         TEXT,
  email         TEXT,
  is_approved   BOOLEAN NOT NULL DEFAULT FALSE,
  reviewed_at   TIMESTAMPTZ,  -- ADDED: set when an admin approves or rejects, so rejected agencies leave the pending list
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email           TEXT UNIQUE NOT NULL,
  phone           TEXT,
  password_hash   TEXT NOT NULL,
  full_name       TEXT NOT NULL,
  role            user_role NOT NULL,
  agency_id       UUID REFERENCES agencies(id),
  designation     TEXT CHECK (designation IN ('SOCIAL_WORKER','SUPERINTENDENT','MEDICAL_OFFICER')),
  language        TEXT NOT NULL DEFAULT 'en' CHECK (language IN ('en','hi','mr')),
  mfa_secret_enc  TEXT,
  is_active       BOOLEAN NOT NULL DEFAULT TRUE,
  failed_logins   INT NOT NULL DEFAULT 0,
  locked_until    TIMESTAMPTZ,
  consent_at      TIMESTAMPTZ,
  token_version   INT NOT NULL DEFAULT 0, -- ADDED: incremented on logout so old refresh tokens stop working
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK ( (role = 'AGENCY' AND agency_id IS NOT NULL) OR (role <> 'AGENCY') )
);

-- ========== APPLICATIONS (one per parent) ==========
CREATE TABLE applications (
  id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  parent_id            UUID UNIQUE NOT NULL REFERENCES users(id),
  agency_id            UUID REFERENCES agencies(id),
  social_worker_id     UUID REFERENCES users(id),
  status               app_status NOT NULL DEFAULT 'REGISTERED',
  marital_status       TEXT NOT NULL CHECK (marital_status IN ('MARRIED','SINGLE_FEMALE','SINGLE_MALE','DIVORCED','WIDOWED')),
  years_married        NUMERIC(4,1),
  applicant_dob        DATE NOT NULL,
  spouse_dob           DATE,
  existing_children    INT NOT NULL DEFAULT 0,
  annual_income_inr    NUMERIC(12,2),
  state                TEXT NOT NULL,
  district             TEXT NOT NULL,
  address_enc          TEXT,
  pref_age_group       TEXT NOT NULL CHECK (pref_age_group IN ('0-2','2-4','4-8','8-18')),
  pref_gender          TEXT NOT NULL CHECK (pref_gender IN ('ANY','MALE','FEMALE')),
  pref_states          TEXT[] NOT NULL DEFAULT '{}',
  open_special_needs   BOOLEAN NOT NULL DEFAULT FALSE,
  open_siblings        BOOLEAN NOT NULL DEFAULT FALSE,
  registered_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  docs_submitted_at    TIMESTAMPTZ,
  seniority_date       TIMESTAMPTZ,
  hsr_valid_until      DATE,
  referrals_used       INT NOT NULL DEFAULT 0 CHECK (referrals_used BETWEEN 0 AND 3),
  debarred_until       DATE,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_app_status ON applications(status);
CREATE INDEX idx_app_queue  ON applications(status, seniority_date);

-- ========== DOCUMENTS ==========
CREATE TABLE documents (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  application_id   UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
  category         doc_category NOT NULL,
  storage_key      TEXT NOT NULL,
  file_name        TEXT NOT NULL,
  mime_type        TEXT NOT NULL,
  size_bytes       INT  NOT NULL,
  sha256           CHAR(64) NOT NULL,
  ai_detected_type TEXT,
  ai_fields        JSONB NOT NULL DEFAULT '{}',
  ai_confidence    NUMERIC(4,3),
  ai_checks        JSONB NOT NULL DEFAULT '[]',
  status           doc_status NOT NULL DEFAULT 'NEEDS_HUMAN',
  reviewer_id      UUID REFERENCES users(id),
  reviewer_note    TEXT,
  is_current       BOOLEAN NOT NULL DEFAULT TRUE,
  uploaded_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  reviewed_at      TIMESTAMPTZ
);
CREATE INDEX idx_docs_app  ON documents(application_id, category) WHERE is_current;
CREATE INDEX idx_docs_hash ON documents(sha256);

-- ========== HOME STUDY REPORT ==========
CREATE TABLE home_study_reports (
  id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  application_id     UUID UNIQUE NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
  social_worker_id   UUID NOT NULL REFERENCES users(id),
  visit_date         DATE,
  raw_notes          TEXT,
  ai_draft           JSONB,
  final_sections     JSONB NOT NULL DEFAULT '{}',
  missing_items      TEXT[] NOT NULL DEFAULT '{}',
  completeness_score INT NOT NULL DEFAULT 0 CHECK (completeness_score BETWEEN 0 AND 100),
  recommendation     TEXT CHECK (recommendation IN ('SUITABLE','NOT_SUITABLE','NEEDS_MORE_INFO')),
  signed_at          TIMESTAMPTZ,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ========== CHILDREN (identity protected; never public) ==========
CREATE TABLE children (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  agency_id             UUID NOT NULL REFERENCES agencies(id),
  case_code             TEXT UNIQUE NOT NULL,
  name_enc              TEXT NOT NULL,
  dob                   DATE NOT NULL,
  gender                TEXT NOT NULL CHECK (gender IN ('MALE','FEMALE')),
  is_special_needs      BOOLEAN NOT NULL DEFAULT FALSE,
  sibling_group_code    TEXT,
  admitted_on           DATE NOT NULL,
  last_family_visit_on  DATE,
  cwc_lfa_number        TEXT,
  lfa_date              DATE,
  csr_storage_key       TEXT,
  mer_storage_key       TEXT,
  mer_text              TEXT,
  summary_ai_draft      TEXT,
  summary_approved      TEXT,
  summary_approved_by   UUID REFERENCES users(id),
  summary_approved_at   TIMESTAMPTZ,
  status                TEXT NOT NULL DEFAULT 'IN_CARE'
                        CHECK (status IN ('IN_CARE','LEGALLY_FREE','READY','REFERRED','RESERVED','PLACED')),
  created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Referral Lock: a child can be READY or later only when both reports and an approved summary exist
ALTER TABLE children ADD CONSTRAINT referral_lock CHECK (
  status NOT IN ('READY','REFERRED','RESERVED','PLACED')
  OR (csr_storage_key IS NOT NULL AND mer_storage_key IS NOT NULL AND summary_approved IS NOT NULL)
);

-- ========== REFERRALS ==========
CREATE TABLE referrals (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  application_id   UUID NOT NULL REFERENCES applications(id),
  child_id         UUID NOT NULL REFERENCES children(id),
  referral_number  INT NOT NULL CHECK (referral_number BETWEEN 1 AND 3),
  sent_by          UUID NOT NULL REFERENCES users(id),
  starts_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at       TIMESTAMPTZ NOT NULL DEFAULT now() + INTERVAL '48 hours',
  decision         TEXT NOT NULL DEFAULT 'PENDING'
                   CHECK (decision IN ('PENDING','RESERVED','NOT_RESERVED','EXPIRED','ACCEPTED','NOT_ACCEPTED_AFTER_RESERVE')),
  decision_reason  TEXT,
  decided_at       TIMESTAMPTZ
);
CREATE UNIQUE INDEX one_active_referral_per_child ON referrals(child_id) WHERE decision = 'PENDING';

-- ========== LEGAL DEADLINES AND EVENTS ==========
CREATE TABLE case_deadlines (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  application_id  UUID REFERENCES applications(id) ON DELETE CASCADE,
  child_id        UUID REFERENCES children(id),
  agency_id       UUID REFERENCES agencies(id),
  step            deadline_step NOT NULL,
  started_at      TIMESTAMPTZ NOT NULL,
  due_at          TIMESTAMPTZ NOT NULL,
  completed_at    TIMESTAMPTZ,
  state           deadline_state NOT NULL DEFAULT 'OPEN',
  risk_score      NUMERIC(4,3),
  risk_reasons    JSONB NOT NULL DEFAULT '[]',
  risk_updated_at TIMESTAMPTZ
);
CREATE INDEX idx_deadlines_open ON case_deadlines(state, due_at);
CREATE INDEX idx_deadlines_app ON case_deadlines(application_id); -- ADDED: case pages look up deadlines by application

CREATE TABLE legal_events (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  application_id  UUID NOT NULL REFERENCES applications(id),
  event_type      TEXT NOT NULL CHECK (event_type IN ('MATCHED','FOSTER_CARE','DM_FILED','DM_ORDER','BIRTH_CERT')),
  event_date      DATE NOT NULL,
  reference_no    TEXT,
  recorded_by     UUID NOT NULL REFERENCES users(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE followups (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  application_id  UUID NOT NULL REFERENCES applications(id),
  visit_number    INT NOT NULL CHECK (visit_number BETWEEN 1 AND 4),
  due_date        DATE NOT NULL,
  visited_on      DATE,
  raw_notes       TEXT,
  ai_draft        JSONB,
  final_report    JSONB,
  concern_flags   TEXT[] NOT NULL DEFAULT '{}',
  signed_by       UUID REFERENCES users(id),
  UNIQUE (application_id, visit_number)
);

-- ========== COMMUNICATION ==========
CREATE TABLE messages (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  application_id  UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
  sender_id       UUID NOT NULL REFERENCES users(id),
  body            TEXT NOT NULL CHECK (length(body) <= 4000),
  from_assistant_escalation BOOLEAN NOT NULL DEFAULT FALSE,
  read_at         TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_messages_app ON messages(application_id, created_at); -- ADDED

CREATE TABLE notifications (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     UUID NOT NULL REFERENCES users(id),
  type        TEXT NOT NULL,
  title       TEXT NOT NULL,
  body        TEXT NOT NULL,
  link        TEXT,
  read_at     TIMESTAMPTZ,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_notifications_user ON notifications(user_id, created_at DESC); -- ADDED

-- ========== AI ==========
CREATE TABLE knowledge_chunks (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  source_title TEXT NOT NULL,
  citation     TEXT NOT NULL,
  source_url   TEXT,
  language     TEXT NOT NULL DEFAULT 'en',
  content      TEXT NOT NULL,
  embedding    vector(768) NOT NULL,
  embed_model  TEXT NOT NULL DEFAULT 'gemini-embedding-001', -- ADDED: which model produced the embedding, so chunks can be re-embedded
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_chunks_embedding ON knowledge_chunks USING hnsw (embedding vector_cosine_ops);

CREATE TABLE ai_runs (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  feature        TEXT NOT NULL CHECK (feature IN ('DOCCHECK','ASSISTANT','HSR_DRAFT','FOLLOWUP_DRAFT','MED_SUMMARY','SCAM_CHECK','RISK_SCORE')),
  actor_id       UUID REFERENCES users(id),
  target_type    TEXT,
  target_id      UUID,
  model          TEXT NOT NULL,
  latency_ms     INT,
  output         JSONB NOT NULL,
  confidence     NUMERIC(4,3),
  human_decision TEXT CHECK (human_decision IN ('ACCEPTED','EDITED','REJECTED')),
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE scam_reports (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  reporter_id UUID REFERENCES users(id),
  text        TEXT NOT NULL,
  verdict     TEXT NOT NULL CHECK (verdict IN ('LIKELY_SCAM','SUSPICIOUS','LOOKS_SAFE')),
  reasons     JSONB NOT NULL DEFAULT '[]',
  reviewed    BOOLEAN NOT NULL DEFAULT FALSE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ========== AUDIT LOG (hash chain) ==========
CREATE TABLE audit_logs (
  id           BIGSERIAL PRIMARY KEY,
  actor_id     UUID REFERENCES users(id),
  actor_role   user_role,
  action       TEXT NOT NULL,
  target_type  TEXT NOT NULL,
  target_id    TEXT NOT NULL,
  ip           TEXT,
  meta         JSONB NOT NULL DEFAULT '{}',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  prev_hash    CHAR(64) NOT NULL,
  hash         CHAR(64) NOT NULL
);
CREATE INDEX idx_audit_created ON audit_logs(created_at); -- ADDED

-- ADDED: Neon gives one owner role, so "REVOKE UPDATE, DELETE" cannot separate the app user.
-- This trigger enforces the same rule: audit entries can never be updated or deleted.
-- +goose StatementBegin
CREATE FUNCTION audit_logs_append_only() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'audit_logs is append-only';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER audit_logs_no_update BEFORE UPDATE OR DELETE ON audit_logs
  FOR EACH ROW EXECUTE FUNCTION audit_logs_append_only();

-- +goose Down
DROP TABLE IF EXISTS audit_logs, scam_reports, ai_runs, knowledge_chunks, notifications, messages, followups,
  legal_events, case_deadlines, referrals, children, home_study_reports, documents, applications, users, agencies CASCADE;
DROP FUNCTION IF EXISTS audit_logs_append_only();
DROP TYPE IF EXISTS deadline_state, deadline_step, doc_status, doc_category, app_status, user_role;
