"use client";

import * as React from "react";
import { CheckCircle2, Info, Sparkles, X, XCircle, AlertTriangle } from "lucide-react";
import { cn } from "@/lib/utils/cn";

type ToastTone = "success" | "error" | "info" | "warning" | "ai";
interface ToastItem { id: number; tone: ToastTone; title: string; body?: string }

const Ctx = React.createContext<{ toast: (t: Omit<ToastItem, "id">) => void } | null>(null);

export function useToast() {
  const c = React.useContext(Ctx);
  if (!c) throw new Error("useToast must be used inside ToastProvider");
  return c.toast;
}

const toneBg: Record<ToastTone, string> = { success: "bg-mint", error: "bg-pink", info: "bg-sky", warning: "bg-sun", ai: "bg-lavender" };
const toneIcon = { success: CheckCircle2, error: XCircle, info: Info, warning: AlertTriangle, ai: Sparkles } as const;

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = React.useState<ToastItem[]>([]);
  const idRef = React.useRef(0);

  const dismiss = React.useCallback((id: number) => setItems((s) => s.filter((i) => i.id !== id)), []);
  const toast = React.useCallback((t: Omit<ToastItem, "id">) => {
    const id = ++idRef.current;
    setItems((s) => [...s, { ...t, id }]);
    setTimeout(() => dismiss(id), t.tone === "error" ? 8000 : 5000);
  }, [dismiss]);

  return (
    <Ctx.Provider value={{ toast }}>
      {children}
      <div className="pointer-events-none fixed inset-x-3 top-3 z-[90] flex flex-col items-end gap-2 sm:inset-x-auto sm:right-4 sm:top-4" aria-live="polite">
        {items.map((i) => {
          const Icon = toneIcon[i.tone];
          return (
            <div
              key={i.id}
              role={i.tone === "error" ? "alert" : "status"}
              className="pointer-events-auto flex w-full max-w-sm items-start gap-3 rounded-[22px] border-2 border-ink bg-white p-3 pr-2 shadow-pop-lg"
            >
              <span className={cn("grid h-9 w-9 shrink-0 place-items-center rounded-full border-2 border-ink", toneBg[i.tone])}>
                <Icon className="h-4 w-4" aria-hidden />
              </span>
              <div className="min-w-0 flex-1 py-0.5">
                <p className="text-sm font-bold">{i.title}</p>
                {i.body && <p className="mt-0.5 text-xs text-ink-soft">{i.body}</p>}
              </div>
              <button type="button" onClick={() => dismiss(i.id)} aria-label="Dismiss" className="grid h-8 w-8 place-items-center rounded-full hover:bg-ink/5">
                <X className="h-4 w-4" aria-hidden />
              </button>
            </div>
          );
        })}
      </div>
    </Ctx.Provider>
  );
}
