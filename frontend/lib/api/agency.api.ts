import { api, uploadForm } from "./client";
import type {
  AgencyCaseDetail, AgencyCaseListItem, ChildDetail, ChildListItem, CaseDocument, HsrSections, LegalEventType,
} from "@/types";

export const agencyApi = {
  cases: (params: { status?: string; sort?: string }, signal?: AbortSignal) =>
    api<AgencyCaseListItem[]>("/agency/cases", { query: { sort: "risk", ...params }, signal }),
  caseDetail: (id: string, signal?: AbortSignal) => api<AgencyCaseDetail>(`/agency/cases/${id}`, { signal }),
  reviewDocument: (id: string, body: { decision: "VERIFIED" } | { decision: "REUPLOAD_REQUESTED"; note: string }) =>
    api<{ document: CaseDocument; application_status: string }>(`/agency/documents/${id}/review`, { method: "PATCH", body }),
  verifyAll: (id: string) => api<{ status: string; hsr_due_at: string }>(`/agency/cases/${id}/verify-all`, { method: "POST" }),
  saveHsr: (applicationId: string, body: { visit_date: string; final_sections: Partial<HsrSections>; recommendation?: string }) =>
    api<{ ok: boolean }>(`/agency/hsr/${applicationId}`, { method: "PUT", body }),
  signHsr: (applicationId: string, ai_run_id: string) =>
    api<{ status: string; hsr_valid_until: string }>(`/agency/hsr/${applicationId}/sign`, { method: "POST", body: { ai_run_id } }),
  recordEvent: (id: string, body: { event_type: LegalEventType; event_date: string; reference_no?: string }) =>
    api<{ status: string; next_deadline?: unknown }>(`/agency/cases/${id}/events`, { method: "POST", body }),
  signFollowup: (id: string, body: { final_report: Record<string, string>; visited_on: string }) =>
    api<{ ok: boolean }>(`/agency/followups/${id}/sign`, { method: "POST", body }),

  addChild: (body: { name: string; dob: string; gender: string; is_special_needs: boolean; admitted_on: string; cwc_lfa_number: string; lfa_date: string }) =>
    api<{ child_id: string; case_code: string }>("/agency/children", { method: "POST", body }),
  uploadReports: (id: string, csr: File, mer: File, opts: { onProgress?: (f: number) => void }) => {
    const form = new FormData();
    form.append("csr", csr);
    form.append("mer", mer);
    return uploadForm<{ ok: boolean; mer_text_extracted: boolean }>(`/agency/children/${id}/reports`, form, opts);
  },
  approveSummary: (id: string, body: { approved_text: string; ai_run_id: string }) =>
    api<{ child_status: string }>(`/agency/children/${id}/summary/approve`, { method: "POST", body }),
  nextParent: (id: string) =>
    api<{ application_id: string; parent_name: string; seniority_date: string; matched_on: string[] }>(`/agency/children/${id}/next-parent`),
  sendReferral: (body: { child_id: string; application_id: string }) =>
    api<{ referral_id: string; expires_at: string }>("/agency/referrals", { method: "POST", body }),

  // PROPOSED, not in guide section 15.6 (see docs/frontend-backend-contract.md)
  children: (signal?: AbortSignal) => api<ChildListItem[]>("/agency/children", { signal }),
  child: (id: string, signal?: AbortSignal) => api<ChildDetail>(`/agency/children/${id}`, { signal }),
};
