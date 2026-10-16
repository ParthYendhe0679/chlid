"use client";

import { useQuery } from "@tanstack/react-query";
import { useLocale, useTranslations } from "next-intl";
import { Card } from "@/components/ui/card";
import { ErrorState, InlineAlert, Skeleton } from "@/components/ui/feedback";
import { publicApi } from "@/lib/api/public.api";
import { formatINR } from "@/lib/utils/format";

/** Fee values always come from the server (GET /public/fees); nothing is hardcoded here. */
export function FeeCard() {
  const t = useTranslations("fees");
  const locale = useLocale();
  const q = useQuery({ queryKey: ["fees"], queryFn: ({ signal }) => publicApi.fees(signal), staleTime: 10 * 60_000 });

  if (q.isPending) return <Skeleton className="h-72" />;
  if (q.isError) return <ErrorState error={q.error} onRetry={() => q.refetch()} />;
  const { items, total_single_child, notes } = q.data;

  return (
    <div className="grid gap-6 lg:grid-cols-[1.2fr_1fr]">
      <Card>
        <ul className="divide-y divide-line">
          {items.map((it) => (
            <li key={it.name} className="flex items-center justify-between gap-4 py-4">
              <span className="font-semibold">{it.name}</span>
              <span className="tabular font-display text-lg font-bold">{formatINR(it.amount, locale)}</span>
            </li>
          ))}
        </ul>
        <div className="mt-2 flex items-center justify-between rounded-2xl bg-lavender-soft p-4">
          <span className="font-bold">{t("total")}</span>
          <span className="tabular font-display text-2xl font-extrabold text-purple-deep">{formatINR(total_single_child, locale)}</span>
        </div>
      </Card>
      <div className="space-y-4">
        <Card variant="sun">
          <h2 className="text-lg font-bold">{t("noExtraTitle")}</h2>
          <ul className="mt-2 list-disc space-y-1 pl-5 text-sm text-ink-soft">
            {notes.map((n) => <li key={n}>{n}</li>)}
          </ul>
        </Card>
        <InlineAlert tone="warning" title={t("warnTitle")}>{t("warnBody")}</InlineAlert>
      </div>
    </div>
  );
}
