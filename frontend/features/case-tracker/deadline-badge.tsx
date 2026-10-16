"use client";

import { AlarmClock } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { Badge } from "@/components/ui/feedback";
import { formatDate } from "@/lib/utils/format";
import type { Deadline } from "@/types";

/** Shows days left from the server value (never recalculated) with the legal rule on hover/focus. */
export function DeadlineBadge({ deadline, showStep = true }: { deadline: Pick<Deadline, "step" | "due_at" | "days_left">; showStep?: boolean }) {
  const t = useTranslations("deadline");
  const locale = useLocale();
  const d = deadline.days_left;
  const tone = d < 0 ? "danger" : d <= 7 ? "warning" : "success";
  const label = d < 0 ? t("overdueBy", { days: Math.abs(d) }) : d === 0 ? t("dueToday") : t("daysLeft", { days: d });
  const rule = t.has(`rule.${deadline.step}`) ? t(`rule.${deadline.step}`) : undefined;
  return (
    <span title={rule} className="inline-flex flex-wrap items-center gap-1.5">
      <Badge tone={tone}>
        <AlarmClock className="h-3 w-3" aria-hidden />
        <span className="tabular">{label}</span>
      </Badge>
      {showStep && t.has(`step.${deadline.step}`) && <span className="text-xs font-medium text-muted">{t(`step.${deadline.step}`)}</span>}
      <span className="sr-only">{t("dueOn", { date: formatDate(deadline.due_at, locale) })}{rule ? `. ${rule}` : ""}</span>
    </span>
  );
}
