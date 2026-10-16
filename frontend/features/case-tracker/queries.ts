import { useQuery } from "@tanstack/react-query";
import { applicationsApi } from "@/lib/api/applications.api";
import { ApiError } from "@/lib/api/errors";

export const qk = {
  application: ["application", "me"] as const,
  tracker: ["application", "tracker"] as const,
  referral: ["referral", "active"] as const,
  followups: ["followups", "me"] as const,
  messages: (id: string) => ["messages", id] as const,
};

/** The parent's application, or null when none has been created yet (server answers 404). */
export function useMyApplication() {
  return useQuery({
    queryKey: qk.application,
    queryFn: async ({ signal }) => {
      try {
        return await applicationsApi.me(signal);
      } catch (e) {
        if (e instanceof ApiError && e.code === "NOT_FOUND") return null;
        throw e;
      }
    },
  });
}

export function useTracker(enabled = true) {
  return useQuery({ queryKey: qk.tracker, queryFn: ({ signal }) => applicationsApi.tracker(signal), enabled });
}
