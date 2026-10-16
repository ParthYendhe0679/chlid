import { api, uploadForm } from "./client";
import type {
  ActiveReferral, ApplicationInput, ApplicationMe, DocCategory, Followup, Message,
  ReferralDecisionResponse, Tracker, UploadResult,
} from "@/types";

export const applicationsApi = {
  create: (body: ApplicationInput) =>
    api<{ application_id: string; status: string; doc_deadline: string }>("/applications", { method: "POST", body }),
  me: (signal?: AbortSignal) => api<ApplicationMe>("/applications/me", { signal }),
  update: (body: Partial<ApplicationInput>) => api<ApplicationMe>("/applications/me", { method: "PATCH", body }),
  tracker: (signal?: AbortSignal) => api<Tracker>("/applications/me/tracker", { signal }),
  uploadDocument: (category: DocCategory, file: File, opts: { onProgress?: (f: number) => void; signal?: AbortSignal }) => {
    const form = new FormData();
    form.append("category", category);
    form.append("file", file);
    return uploadForm<UploadResult>("/applications/me/documents", form, opts);
  },
  submit: () => api<{ status: string; seniority_date: string }>("/applications/me/submit", { method: "POST" }),
  documentUrl: (id: string) => api<{ url: string; expires_in: number }>(`/documents/${id}/url`),
  activeReferral: (signal?: AbortSignal) => api<ActiveReferral | null>("/referrals/active", { signal }),
  decide: (id: string, body: { decision: "RESERVED" } | { decision: "NOT_RESERVED"; reason: string }) =>
    api<ReferralDecisionResponse>(`/referrals/${id}/decision`, { method: "POST", body }),
  followups: (signal?: AbortSignal) => api<Followup[]>("/followups/me", { signal }),
};

export const messagesApi = {
  list: (applicationId: string, signal?: AbortSignal) => api<Message[]>(`/applications/${applicationId}/messages`, { signal }),
  send: (applicationId: string, body: string) =>
    api<Message>(`/applications/${applicationId}/messages`, { method: "POST", body: { body } }),
};
