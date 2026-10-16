"use client";

import * as React from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { authApi, type LoginInput, type RegisterInput } from "@/lib/api/auth.api";
import { ApiError } from "@/lib/api/errors";
import { SESSION_EXPIRED_EVENT } from "@/lib/api/client";
import { setLocaleCookie } from "@/lib/utils/locale";
import type { SessionUser } from "@/types";

interface AuthCtx {
  user: SessionUser | null;
  status: "loading" | "authed" | "anon";
  sessionExpired: boolean;
  login: (i: LoginInput) => Promise<SessionUser>;
  register: (i: RegisterInput) => Promise<SessionUser>;
  logout: () => Promise<void>;
}

const Ctx = React.createContext<AuthCtx | null>(null);

export function useAuth() {
  const c = React.useContext(Ctx);
  if (!c) throw new Error("useAuth must be used inside AuthProvider");
  return c;
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const qc = useQueryClient();
  const [expired, setExpired] = React.useState(false);

  // Session restoration: the cookie is httpOnly, so /auth/me is the only way to know who is logged in.
  const me = useQuery({
    queryKey: ["me"],
    queryFn: async ({ signal }) => {
      try {
        return await authApi.me(signal);
      } catch (e) {
        if (e instanceof ApiError && e.isAuth) return null;
        throw e;
      }
    },
    staleTime: 5 * 60_000,
    retry: false,
  });

  React.useEffect(() => {
    const h = () => { setExpired(true); qc.clear(); qc.setQueryData(["me"], null); };
    window.addEventListener(SESSION_EXPIRED_EVENT, h);
    return () => window.removeEventListener(SESSION_EXPIRED_EVENT, h);
  }, [qc]);

  const value = React.useMemo<AuthCtx>(() => {
    const user = me.data ?? null;
    const adopt = (u: SessionUser) => {
      setExpired(false);
      qc.setQueryData(["me"], u);
      setLocaleCookie(u.language);
      return u;
    };
    return {
      user,
      status: me.isPending ? "loading" : user ? "authed" : "anon",
      sessionExpired: expired,
      login: async (i) => adopt((await authApi.login(i)).user),
      register: async (i) => adopt((await authApi.register(i)).user),
      logout: async () => {
        try { await authApi.logout(); } finally { qc.clear(); qc.setQueryData(["me"], null); }
      },
    };
  }, [me.data, me.isPending, expired, qc]);

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}
