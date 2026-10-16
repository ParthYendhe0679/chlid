import { api } from "./client";
import type { Lang, SessionUser } from "@/types";

export interface LoginInput { email: string; password: string; otp?: string }
export interface RegisterInput {
  email: string; phone: string; password: string; full_name: string; language: Lang; consent: true;
}
type UserEnvelope = { user: SessionUser };

export const authApi = {
  login: (input: LoginInput) => api<UserEnvelope>("/auth/login", { method: "POST", body: input, skipRefresh: true }),
  register: (input: RegisterInput) => api<UserEnvelope>("/auth/register", { method: "POST", body: input, skipRefresh: true }),
  logout: () => api<{ ok: boolean }>("/auth/logout", { method: "POST", skipRefresh: true }),
  /** Guide 15.3 does not state the envelope for /auth/me; both shapes are accepted. */
  me: async (signal?: AbortSignal): Promise<SessionUser> => {
    const r = await api<UserEnvelope | SessionUser>("/auth/me", { signal });
    return "user" in r ? r.user : r;
  },
};
