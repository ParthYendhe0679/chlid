import { api } from "./client";
import type { AssistantResponse, FollowupDraft, HsrDraft, Lang } from "@/types";

export const aiApi = {
  assistant: (question: string, language: Lang, signal?: AbortSignal) =>
    api<AssistantResponse>("/ai/assistant", { method: "POST", body: { question, language }, signal, timeoutMs: 45_000 }),
  escalate: (question: string) => api<{ message_id: string }>("/ai/assistant/escalate", { method: "POST", body: { question } }),
  hsrDraft: (body: { application_id: string; visit_date: string; raw_notes: string }, signal?: AbortSignal) =>
    api<HsrDraft>("/ai/hsr/draft", { method: "POST", body, signal, timeoutMs: 60_000 }),
  followupDraft: (body: { followup_id: string; raw_notes: string }, signal?: AbortSignal) =>
    api<FollowupDraft>("/ai/followup/draft", { method: "POST", body, signal, timeoutMs: 60_000 }),
  medicalSummary: (body: { child_id: string; language: Lang }, signal?: AbortSignal) =>
    api<{ draft: string; ai_run_id: string }>("/ai/medical-summary", { method: "POST", body, signal, timeoutMs: 60_000 }),
};
