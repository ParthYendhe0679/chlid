import { api } from "./client";
import type { AgencyListing, EligibilityRequest, EligibilityResponse, FeesResponse, Lang, ScamCheckResponse } from "@/types";

export const publicApi = {
  eligibility: (body: EligibilityRequest) => api<EligibilityResponse>("/public/eligibility", { method: "POST", body }),
  fees: (signal?: AbortSignal) => api<FeesResponse>("/public/fees", { signal }),
  agencies: (q: string, state: string, signal?: AbortSignal) =>
    api<{ agencies: AgencyListing[] }>("/public/agencies", { query: { q, state }, signal }),
  scamCheck: (text: string, language: Lang) => api<ScamCheckResponse>("/public/scam-check", { method: "POST", body: { text, language } }),
  scamReport: (report_token: string) => api<{ ok: boolean }>("/public/scam-report", { method: "POST", body: { report_token } }),
};
