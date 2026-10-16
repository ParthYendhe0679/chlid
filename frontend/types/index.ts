// Shared types. Aligned with section 15 of allaboutourproject.md.
// Types marked PROPOSED have no contract in the guide yet; see docs/frontend-backend-contract.md.

export type Role = "PARENT" | "AGENCY" | "ADMIN";
export type Lang = "en" | "hi" | "mr";
export type AppStatus =
  | "REGISTERED" | "DOCS_SUBMITTED" | "DOCS_CORRECTION" | "HSR_IN_PROGRESS" | "WAITING"
  | "REFERRED" | "RESERVED" | "MATCHED_FOSTER_CARE" | "DM_ORDER_PENDING"
  | "ADOPTED_FOLLOWUP" | "COMPLETED" | "LAPSED" | "DEBARRED" | "WITHDRAWN";
export type CheckResult = "PASS" | "WARN" | "FAIL";
export type DocCategory =
  | "PHOTO" | "IDENTITY" | "BIRTH_PROOF" | "RESIDENCE" | "INCOME" | "MEDICAL"
  | "MARRIAGE" | "DIVORCE_OR_DEATH" | "REFERENCE" | "CHILD_CONSENT";
export type DocStatus =
  | "AI_PASS" | "AI_WARN" | "AI_FAIL" | "NEEDS_HUMAN" | "VERIFIED" | "REUPLOAD_REQUESTED";
export type DeadlineStep =
  | "DOC_UPLOAD_30D" | "HSR_60D" | "CSR_MER_10D" | "RESERVE_48H" | "MATCHING_30D"
  | "FOSTER_CARE_10D" | "DM_FILING_10D" | "DM_ORDER_60D" | "FOLLOWUP_VISIT";
export type MaritalStatus = "MARRIED" | "SINGLE_FEMALE" | "SINGLE_MALE" | "DIVORCED" | "WIDOWED";
export type AgeGroup = "0-2" | "2-4" | "4-8" | "8-18";
export type Designation = "SOCIAL_WORKER" | "SUPERINTENDENT" | "MEDICAL_OFFICER";

export interface DocCheck { code: string; result: CheckResult; message: string }
export interface Deadline {
  step: string;
  due_at: string;
  days_left: number;
  risk_score?: number | null;
  risk_reasons?: string[];
}
export interface Citation { source_title: string; citation: string; source_url?: string }
export interface ApiErrorBody { error: { code: string; message: string; request_id: string } }

export interface SessionUser {
  id: string;
  role: Role;
  agency_id?: string | null;
  full_name: string;
  language: Lang;
  designation?: Designation | null; // PROPOSED: guide stores it on users but /auth/me does not list it
}

// ---- public ----
export interface EligibilityRequest {
  marital_status: MaritalStatus;
  applicant_dob: string;
  spouse_dob?: string;
  years_married?: number;
  existing_children: number;
}
export interface EligibilityResponse {
  eligible: boolean;
  allowed_age_groups: AgeGroup[];
  reasons: { code: string; message: string }[];
}
export interface FeesResponse {
  items: { name: string; amount: number }[];
  total_single_child: number;
  notes: string[];
}
export interface AgencyListing {
  name: string; reg_number: string; type: "SAA" | "DCPU"; district: string; state: string; verified: boolean;
}
export interface ScamCheckResponse {
  verdict: "LIKELY_SCAM" | "SUSPICIOUS" | "LOOKS_SAFE";
  reasons: string[];
  advice: string;
  report_token: string;
}

// ---- parent ----
export interface ApplicationInput {
  marital_status: MaritalStatus;
  years_married?: number;
  applicant_dob: string;
  spouse_dob?: string;
  existing_children: number;
  annual_income_inr?: number;
  state: string;
  district: string;
  address?: string; // PROPOSED: guide stores address_enc but does not list the request field name
  pref_age_group: AgeGroup;
  pref_gender: "ANY" | "MALE" | "FEMALE";
  pref_states: string[];
  open_special_needs: boolean;
  open_siblings: boolean;
}
export interface RequiredDoc { category: DocCategory; uploaded: boolean; status?: DocStatus | null; document_id?: string; reviewer_note?: string | null }
export interface ApplicationMe extends Partial<ApplicationInput> {
  id: string;
  status: AppStatus;
  registered_at?: string;
  doc_deadline?: string | null;
  seniority_date?: string | null;
  referrals_used?: number;
  debarred_until?: string | null;
  required_docs: RequiredDoc[];
}
export interface TrackerStage { key: string; title: string; done: boolean; date?: string | null }
export interface Tracker {
  status: AppStatus;
  stage_index: number;
  stages: TrackerStage[];
  current_deadline?: Deadline | null;
  next_step_text: string;
  referrals_used: number;
}
export interface UploadResult {
  document_id: string;
  status: DocStatus;
  detected_type?: string | null;
  confidence?: number | null;
  checks: DocCheck[];
}
export interface ActiveReferral {
  referral_id: string;
  referral_number: number;
  expires_at: string;
  child: {
    case_code: string; age_text: string; gender: "MALE" | "FEMALE"; is_special_needs: boolean;
    summary_approved: string; csr_url: string; mer_url: string;
  };
}
export interface ReferralDecisionResponse { status: string; warning?: string }
export interface Followup { visit_number: number; due_date: string; visited_on?: string | null }
export interface Message {
  id: string; sender_name: string; sender_role: Role; body: string; created_at: string;
}

// ---- AI ----
export interface AssistantResponse {
  answer: string;
  citations: Citation[];
  grounded: boolean;
  is_about_own_case: boolean;
  offer_escalation: boolean;
}
export interface HsrSections {
  family_background: string; home_environment: string; health: string; finances: string;
  motivation_readiness: string; parenting_plan: string; support_system: string; observations: string;
}
export interface HsrDraft { sections: HsrSections; missing_items: string[]; completeness_score: number; ai_run_id: string }
export interface FollowupDraft { sections: Record<string, string>; concern_flags: string[]; ai_run_id: string }

// ---- agency ----
export interface AgencyCaseListItem {
  application_id: string; parent_name: string; status: AppStatus; current_deadline?: Deadline | null;
}
export interface CaseDocument {
  id: string; category: DocCategory; file_name: string; status: DocStatus; ai_detected_type?: string | null;
  ai_confidence?: number | null; ai_fields: Record<string, string | null>; ai_checks: DocCheck[];
  reviewer_note?: string | null; uploaded_at: string;
}
export interface AgencyCaseDetail {
  application_id: string; parent_name: string; status: AppStatus;
  documents: CaseDocument[];
  hsr?: {
    visit_date?: string | null; raw_notes?: string | null; ai_draft?: HsrDraft | null;
    final_sections: Partial<HsrSections>; missing_items: string[]; completeness_score: number;
    recommendation?: "SUITABLE" | "NOT_SUITABLE" | "NEEDS_MORE_INFO" | null; signed_at?: string | null;
    ai_run_id?: string | null; // PROPOSED: needed to sign after a page reload
  } | null;
  timeline: { event_type: string; event_date: string; reference_no?: string | null }[];
  followups: (Followup & { id: string; concern_flags?: string[] })[];
  current_deadline?: Deadline | null;
}
export type LegalEventType = "MATCHED" | "FOSTER_CARE" | "DM_FILED" | "DM_ORDER" | "BIRTH_CERT";
// PROPOSED list/detail shapes for children (guide has only create/report/summary endpoints)
export interface ChildListItem {
  id: string; case_code: string; status: "IN_CARE" | "LEGALLY_FREE" | "READY" | "REFERRED" | "RESERVED" | "PLACED";
  gender: "MALE" | "FEMALE"; is_special_needs: boolean; age_text: string;
}
export interface ChildDetail extends ChildListItem {
  has_csr: boolean; has_mer: boolean; summary_ai_draft?: string | null; summary_approved?: string | null;
}

// ---- admin ----
export interface RadarItem {
  case_ref: string; agency: string; step: string; due_at: string; days_left: number;
  risk_score?: number | null; risk_reasons: string[];
  level?: "OVERDUE" | "HIGH_RISK" | "ON_TRACK"; // PROPOSED: guide gives only summary counts
}
export interface RadarResponse {
  summary: { overdue: number; high_risk: number; on_track: number };
  items: RadarItem[];
}
export interface AgencyScore { agency: string; avg_days: Record<string, number>; late_rate: number; open_cases: number }
export interface ChildAlert { case_code: string; agency: string; reason: "NO_FAMILY_VISIT_12M" | "REPORTS_MISSING_10D"; days: number }
export interface AdminAgency { id: string; name: string; reg_number: string; agency_type: "SAA" | "DCPU"; state: string; district: string; is_approved: boolean }
export interface ScamReport { id: string; text: string; verdict: string; reasons: string[]; reviewed: boolean; created_at: string }
export interface AuditEntry {
  id: number; actor_id?: string | null; actor_role?: Role | null; action: string; target_type: string;
  target_id: string; created_at: string; meta?: Record<string, unknown>;
}
export interface AuditVerify { valid: boolean; rows_checked?: number; broken_at_id?: number }
