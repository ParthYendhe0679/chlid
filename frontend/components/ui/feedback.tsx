"use client";

import * as React from "react";
import { AlertTriangle, CheckCircle2, Info, RefreshCw, Sparkles, XCircle, Inbox, ShieldAlert } from "lucide-react";
import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils/cn";
import { Button } from "./button";
import { ApiError } from "@/lib/api/errors";
import { ArtTile } from "./art";

export { ArtTile };

type Tone = "success" | "warning" | "danger" | "info" | "purple" | "neutral";
const toneClass: Record<Tone, string> = {
  success: "bg-success-soft text-success",
  warning: "bg-warning-soft text-warning",
  danger: "bg-danger-soft text-danger",
  info: "bg-info-soft text-info",
  purple: "bg-lavender-soft text-purple-deep",
  neutral: "bg-ink/5 text-ink-soft",
};
const dotClass: Record<Tone, string> = {
  success: "bg-success", warning: "bg-warning", danger: "bg-danger", info: "bg-info", purple: "bg-purple", neutral: "bg-faint",
};

export function Badge({ tone = "neutral", children, dot = true, className }: { tone?: Tone; children: React.ReactNode; dot?: boolean; className?: string }) {
  return (
    <span className={cn("inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-bold tracking-wide", toneClass[tone], className)}>
      {dot && <span className={cn("h-1.5 w-1.5 rounded-full", dotClass[tone])} aria-hidden />}
      {children}
    </span>
  );
}

export function Skeleton({ className }: { className?: string }) {
  return <div aria-hidden className={cn("skeleton rounded-2xl", className)} />;
}

export function PageSkeleton() {
  return (
    <div role="status" aria-busy="true" className="space-y-4">
      <span className="sr-only">Loading</span>
      <Skeleton className="h-10 w-2/3" />
      <div className="grid gap-4 sm:grid-cols-3">
        <Skeleton className="h-28" /><Skeleton className="h-28" /><Skeleton className="h-28" />
      </div>
      <Skeleton className="h-64" />
    </div>
  );
}

export function EmptyState({ title, body, action, icon: Icon = Inbox }: { title: string; body?: string; action?: React.ReactNode; icon?: React.ComponentType<{ className?: string }> }) {
  return (
    <div className="rounded-[28px] bg-white/60 p-8 text-center sm:p-14">
      <span className="relative mx-auto mb-4 grid h-16 w-16 place-items-center overflow-hidden rounded-[30%] bg-lavender shadow-clay-sm" aria-hidden>
        <span className="absolute inset-x-1 top-0.5 h-1/2 rounded-full bg-white/45 blur-[2px]" />
        <Icon className="relative h-8 w-8 text-purple-deep" />
      </span>
      <p className="font-display text-lg font-bold">{title}</p>
      {body && <p className="mx-auto mt-1 max-w-md text-sm text-muted">{body}</p>}
      {action && <div className="mt-5">{action}</div>}
    </div>
  );
}

/** Maps an API error to a user-facing message and a distinct presentation (permission, session, network, generic). */
export function ErrorState({ error, onRetry, compact }: { error: unknown; onRetry?: () => void; compact?: boolean }) {
  const t = useTranslations("common");
  const e = error instanceof ApiError ? error : null;
  const forbidden = e?.isForbidden;
  const msg = forbidden
    ? t("errors.forbidden")
    : e?.code === "NETWORK_ERROR" ? t("errors.network")
    : e?.code === "TIMEOUT" ? t("errors.timeout")
    : e?.code === "AI_UNAVAILABLE" ? t("errors.aiUnavailable")
    : e?.code === "RATE_LIMITED" ? t("errors.rateLimited")
    : e?.code === "NOT_FOUND" ? t("errors.notFound")
    : e?.message ?? t("errors.generic");
  return (
    <div role="alert" className={cn("rounded-[28px] border-2 border-dashed border-danger/30 bg-danger-soft/40 text-center", compact ? "px-4 py-5" : "px-6 py-10")}>
      {forbidden ? <ShieldAlert className="mx-auto mb-2 h-6 w-6 text-danger" aria-hidden /> : <XCircle className="mx-auto mb-2 h-6 w-6 text-danger" aria-hidden />}
      <p className="font-semibold text-ink">{msg}</p>
      {e && e.message && e.message !== msg && ["FORBIDDEN", "NOT_FOUND", "AI_UNAVAILABLE", "INVALID_TRANSITION"].includes(e.code) && (
        <p className="mx-auto mt-1 max-w-md text-sm text-muted">{e.message}</p>
      )}
      {e?.requestId && <p className="mt-1 font-mono text-[11px] text-muted">ref: {e.requestId}</p>}
      {onRetry && !forbidden && (
        <Button variant="secondary" size="sm" className="mt-4" onClick={onRetry}>
          <RefreshCw className="h-4 w-4" aria-hidden /> {t("retry")}
        </Button>
      )}
    </div>
  );
}

const alertIcon = { success: CheckCircle2, warning: AlertTriangle, danger: XCircle, info: Info, purple: Sparkles, neutral: Info } as const;

export function InlineAlert({ tone = "info", title, children, className }: { tone?: Tone; title?: string; children?: React.ReactNode; className?: string }) {
  const Icon = alertIcon[tone];
  return (
    <div role={tone === "danger" ? "alert" : "status"} className={cn("flex gap-3 rounded-2xl p-3.5 text-sm", toneClass[tone], className)}>
      <Icon className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
      <div>
        {title && <p className="font-bold">{title}</p>}
        {children && <div className={cn("text-ink-soft", title && "mt-0.5")}>{children}</div>}
      </div>
    </div>
  );
}

/** Required on every AI output (section 17.3). */
export function AiLabel({ className }: { className?: string }) {
  const t = useTranslations("common");
  return (
    <span className={cn("inline-flex items-center gap-1.5 rounded-full border-2 border-ink bg-lavender px-2.5 py-0.5 text-[11px] font-bold shadow-[2px_2px_0_#1c1b2b]", className)}>
      <Sparkles className="h-3 w-3" aria-hidden /> {t("aiLabel")}
    </span>
  );
}

export function ProgressBar({ value, label, indeterminate }: { value?: number; label: string; indeterminate?: boolean }) {
  return (
    <div
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={indeterminate ? undefined : Math.round((value ?? 0) * 100)}
      className="relative h-3 overflow-hidden rounded-full bg-ink/10 shadow-inner"
    >
      {indeterminate ? (
        <span className="absolute inset-y-0 left-0 w-1/4 rounded-full bg-purple motion-safe:animate-scan" />
      ) : (
        <span className="block h-full rounded-full bg-gradient-to-r from-lavender to-purple transition-all" style={{ width: `${Math.round((value ?? 0) * 100)}%` }} />
      )}
    </div>
  );
}

export function StatCard({ label, value, hint, icon, tone = "lavender" }: { label: string; value: React.ReactNode; hint?: string; icon: React.ComponentType<{ className?: string }>; tone?: "lavender" | "sun" | "mint" | "sky" | "coral" | "pink" }) {
  return (
    <div className="rounded-[24px] bg-white p-5 shadow-clay-sm">
      <div className="flex items-start justify-between gap-3">
        <p className="text-xs font-bold uppercase tracking-wider text-muted">{label}</p>
        <ArtTile icon={icon} tone={tone} size={40} />
      </div>
      <p className="tabular mt-2 font-display text-[28px] font-extrabold leading-none">{value}</p>
      {hint && <p className="mt-2 text-xs font-medium text-muted">{hint}</p>}
    </div>
  );
}
