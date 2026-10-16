"use client";

import { CalendarCheck, CalendarClock } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/feedback";
import { formatDate } from "@/lib/utils/format";
import type { Followup } from "@/types";

/** Read-only: visits are created by the server when the adoption order is recorded (guide 9). */
export function FollowupSchedule({ items }: { items: Followup[] }) {
  const t = useTranslations("followups");
  const locale = useLocale();
  return (
    <Card variant="mint">
      <h2 className="text-lg font-bold">{t("title")}</h2>
      <p className="mt-1 text-xs text-ink-soft">{t("subtitle")}</p>
      <ul className="mt-4 space-y-2">
        {[...items].sort((a, b) => a.visit_number - b.visit_number).map((f) => (
          <li key={f.visit_number} className="flex items-center justify-between gap-3 rounded-2xl bg-white p-3 text-sm shadow-clay-sm">
            <span className="flex items-center gap-2 font-semibold">
              {f.visited_on ? <CalendarCheck className="h-4 w-4 text-success" aria-hidden /> : <CalendarClock className="h-4 w-4 text-muted" aria-hidden />}
              {t("visit", { n: f.visit_number })}
            </span>
            <span className="flex items-center gap-2">
              <span className="tabular text-xs text-muted">{formatDate(f.visited_on ?? f.due_date, locale)}</span>
              <Badge tone={f.visited_on ? "success" : "info"}>{f.visited_on ? t("done") : t("scheduled")}</Badge>
            </span>
          </li>
        ))}
      </ul>
    </Card>
  );
}
