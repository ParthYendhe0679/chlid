"use client";

import { useTranslations } from "next-intl";
import { Card } from "@/components/ui/card";
import { InlineAlert } from "@/components/ui/feedback";

/**
 * Wait-Time Explainer. There is no backend endpoint for statistics yet (see contract: PROPOSED GET /public/stats).
 * This panel therefore shows only the qualitative public figures quoted in guide section 3.8, with the year,
 * and makes no per-parent prediction. It explains; it does not pressure (guide 4, problem 8).
 */
export function WaitTimeExplainer({ ageGroup, openSpecialNeeds }: { ageGroup: string; openSpecialNeeds: boolean }) {
  const t = useTranslations("waitTime");
  const young = ageGroup === "0-2";
  return (
    <Card variant="sky">
      <h2 className="text-lg font-bold">{t("title")}</h2>
      <ul className="mt-3 space-y-2 text-sm">
        <li className="rounded-2xl bg-white p-3">{t("fact1")}</li>
        <li className="rounded-2xl bg-white p-3">{t("fact2")}</li>
        <li className="rounded-2xl bg-white p-3">{t("fact3")}</li>
      </ul>
      <div className="mt-4 rounded-2xl bg-white p-3 text-sm" aria-live="polite">
        <p className="font-bold">{t("yourChoices")}</p>
        <p className="mt-1 text-ink-soft">{young ? t("youngNote") : t("olderNote")}</p>
        <p className="mt-1 text-ink-soft">{openSpecialNeeds ? t("openNote") : t("closedNote")}</p>
      </div>
      <InlineAlert tone="neutral" className="mt-3">{t("uncertainty")}</InlineAlert>
    </Card>
  );
}
