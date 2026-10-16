"use client";

import { TrendingUp } from "lucide-react";
import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/feedback";

export const HIGH_RISK_THRESHOLD = 0.6; // guide 8.4: High risk = probability >= 0.6

/**
 * Predicted risk, always labelled as a prediction (never a fact). Rendered only when the server supplies a score;
 * a missing score is shown as "not available", never as a made-up number.
 */
export function RiskReasons({ score, reasons, compact }: { score?: number | null; reasons?: string[]; compact?: boolean }) {
  const t = useTranslations("risk");
  if (score === undefined || score === null) {
    return <span className="text-xs text-muted">{t("unavailable")}</span>;
  }
  const high = score >= HIGH_RISK_THRESHOLD;
  return (
    <div className="space-y-1.5">
      <div className="flex flex-wrap items-center gap-2">
        <Badge tone={high ? "danger" : "info"}><TrendingUp className="h-3 w-3" aria-hidden /> {t("predicted")}: <span className="tabular">{score.toFixed(2)}</span></Badge>
        <span className="text-[11px] text-muted">{t("predictionNote")}</span>
      </div>
      {!compact && reasons && reasons.length > 0 && (
        <ul className="list-disc pl-5 text-xs text-ink-soft">{reasons.slice(0, 3).map((r) => <li key={r}>{r}</li>)}</ul>
      )}
      {compact && reasons && reasons.length > 0 && <p className="text-[11px] text-muted">{reasons.slice(0, 3).join(" | ")}</p>}
    </div>
  );
}
