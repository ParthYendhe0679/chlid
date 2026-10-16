"use client";

import { Check } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { cn } from "@/lib/utils/cn";
import { formatDate } from "@/lib/utils/format";
import type { TrackerStage } from "@/types";

/** Renders the stages exactly as returned by the server. Dates shown only when the server provides them. */
export function StageStepper({ stages, currentIndex }: { stages: TrackerStage[]; currentIndex: number }) {
  const t = useTranslations("tracker");
  const locale = useLocale();
  return (
    <ol className="relative space-y-1" aria-label={t("stagesLabel")}>
      {stages.map((s, i) => {
        const current = i === currentIndex && !s.done;
        return (
          <li key={s.key} aria-current={current ? "step" : undefined} className="relative flex gap-4 pb-5 last:pb-0">
            {i < stages.length - 1 && <span aria-hidden className={cn("absolute left-[17px] top-9 h-[calc(100%-2rem)] w-0.5", s.done ? "bg-success" : "bg-ink/10")} />}
            <span
              className={cn(
                "relative grid h-9 w-9 shrink-0 place-items-center rounded-full border-2 text-xs font-bold",
                s.done ? "border-success bg-success text-white" : current ? "border-ink bg-sun shadow-pop" : "border-ink/15 bg-white text-faint",
              )}
            >
              {s.done ? <Check className="h-4 w-4" aria-hidden /> : <span className="tabular">{i + 1}</span>}
            </span>
            <div className="min-w-0 pt-1">
              <p className={cn("font-semibold", !s.done && !current && "text-muted")}>
                {s.title}
                <span className="sr-only"> {s.done ? t("done") : current ? t("current") : t("upcoming")}</span>
              </p>
              {s.date && <p className="text-xs text-muted">{formatDate(s.date, locale)}</p>}
              {current && <p className="mt-0.5 text-xs font-bold text-purple-deep">{t("youAreHere")}</p>}
            </div>
          </li>
        );
      })}
    </ol>
  );
}
