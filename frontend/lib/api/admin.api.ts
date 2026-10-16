import { api } from "./client";
import type { AdminAgency, AgencyScore, AuditEntry, AuditVerify, ChildAlert, RadarResponse, ScamReport } from "@/types";

export const adminApi = {
  radar: (params: { agency_id?: string; step?: string; level?: string }, signal?: AbortSignal) =>
    api<RadarResponse>("/admin/radar", { query: params, signal }),
  recompute: () => api<{ updated: number }>("/admin/radar/recompute", { method: "POST" }),
  scorecard: (signal?: AbortSignal) => api<AgencyScore[]>("/admin/agency-scorecard", { signal }),
  childAlerts: (signal?: AbortSignal) => api<ChildAlert[]>("/admin/child-alerts", { signal }),
  agencies: (approved: boolean, signal?: AbortSignal) => api<AdminAgency[]>("/admin/agencies", { query: { approved }, signal }),
  setApproval: (id: string, is_approved: boolean) => api<AdminAgency>(`/admin/agencies/${id}`, { method: "PATCH", body: { is_approved } }),
  addKnowledge: (body: { source_title: string; source_url?: string; language: string; text: string }) =>
    api<{ chunks_created: number }>("/admin/knowledge", { method: "POST", body, timeoutMs: 120_000 }),
  scamReports: (signal?: AbortSignal) => api<ScamReport[]>("/admin/scam-reports", { signal }),
  audit: (params: { actor?: string; action?: string; from?: string; to?: string }, signal?: AbortSignal) =>
    api<AuditEntry[]>("/admin/audit", { query: params, signal }),
  verifyAudit: () => api<AuditVerify>("/admin/audit/verify", { timeoutMs: 60_000 }),
};
