"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils/cn";

export type TermTag = "LOCAL" | "UPLOAD" | "SERVER" | "CHECK" | "RESULT" | "ERROR" | "INFO";
export interface TermLine { id: number; tag: TermTag; text: string; at: number; tone?: "ok" | "warn" | "fail" }

const tagColor: Record<TermTag, string> = {
  LOCAL: "text-sky",
  UPLOAD: "text-lavender",
  SERVER: "text-sun",
  CHECK: "text-mint",
  RESULT: "text-mint",
  ERROR: "text-pink",
  INFO: "text-white/60",
};
const toneColor = { ok: "text-mint", warn: "text-sun", fail: "text-pink" } as const;

/**
 * Terminal-styled log. Every line is produced from a real signal (local validation, XHR upload progress,
 * the server response). Nothing here is simulated; see DocCheckRun in use-doccheck.ts.
 */
export function TerminalPanel({
  title, lines, busy, indeterminate, progress, elapsedMs,
}: { title: string; lines: TermLine[]; busy: boolean; indeterminate: boolean; progress: number | null; elapsedMs: number }) {
  const t = useTranslations("doccheck");
  const endRef = React.useRef<HTMLDivElement>(null);
  React.useEffect(() => { endRef.current?.scrollIntoView({ block: "nearest" }); }, [lines.length]);
  const start = lines[0]?.at ?? 0;

  return (
    <div className="overflow-hidden rounded-[22px] border-2 border-ink bg-ink text-white shadow-pop">
      <div className="flex items-center justify-between gap-3 border-b border-white/10 bg-white/5 px-4 py-2">
        <div className="flex items-center gap-1.5" aria-hidden>
          <span className="h-3 w-3 rounded-full bg-coral" /><span className="h-3 w-3 rounded-full bg-sun" /><span className="h-3 w-3 rounded-full bg-mint" />
        </div>
        <p className="truncate font-mono text-xs text-white/70">{title}</p>
        <p className="tabular font-mono text-xs text-white/70" aria-label={t("elapsed")}>{(elapsedMs / 1000).toFixed(1)}s</p>
      </div>

      {busy && (
        <div className="relative h-1 overflow-hidden bg-white/10" aria-hidden>
          {indeterminate || progress === null ? (
            <span className="absolute inset-y-0 left-0 w-1/4 bg-purple motion-safe:animate-scan" />
          ) : (
            <span className="block h-full bg-purple transition-all" style={{ width: `${Math.round(progress * 100)}%` }} />
          )}
        </div>
      )}

      <div role="log" aria-live="polite" aria-label={t("logLabel")} className="max-h-72 min-h-40 space-y-1 overflow-y-auto px-4 py-3 font-mono text-[12.5px] leading-relaxed">
        {lines.length === 0 && <p className="text-white/50">{t("waiting")}</p>}
        {lines.map((l) => (
          <p key={l.id} className="break-words">
            <span className="tabular mr-2 text-white/40">+{((l.at - start) / 1000).toFixed(1)}s</span>
            <span className={cn("mr-2 font-bold", tagColor[l.tag])}>[{l.tag}]</span>
            <span className={l.tone ? toneColor[l.tone] : "text-white/90"}>{l.text}</span>
          </p>
        ))}
        {busy && <p className="text-white/70"><span className="mr-2 text-purple">$</span>{t("working")}<span className="ml-0.5 inline-block w-2 bg-white/80 motion-safe:animate-blink">&nbsp;</span></p>}
        <div ref={endRef} />
      </div>
    </div>
  );
}
