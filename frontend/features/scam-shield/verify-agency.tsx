"use client";

import * as React from "react";
import { useQuery } from "@tanstack/react-query";
import { BadgeCheck, Search } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Field, Input } from "@/components/ui/form";
import { Badge, EmptyState, ErrorState, InlineAlert, Skeleton } from "@/components/ui/feedback";
import { publicApi } from "@/lib/api/public.api";

export function VerifyAgency() {
  const t = useTranslations("verify");
  const [q, setQ] = React.useState("");
  const [state, setState] = React.useState("");
  const [submitted, setSubmitted] = React.useState<{ q: string; state: string } | null>(null);

  const query = useQuery({
    queryKey: ["agencies", submitted],
    queryFn: ({ signal }) => publicApi.agencies(submitted!.q, submitted!.state, signal),
    enabled: submitted !== null,
  });

  return (
    <div className="space-y-6">
      <Card>
        <form
          className="grid gap-4 sm:grid-cols-[1fr_1fr_auto] sm:items-end"
          onSubmit={(e) => { e.preventDefault(); setSubmitted({ q: q.trim(), state: state.trim() }); }}
        >
          <Field label={t("name")} htmlFor="agency-q"><Input id="agency-q" value={q} onChange={(e) => setQ(e.target.value)} placeholder={t("namePlaceholder")} /></Field>
          <Field label={t("state")} htmlFor="agency-state"><Input id="agency-state" value={state} onChange={(e) => setState(e.target.value)} placeholder="Maharashtra" /></Field>
          <Button type="submit" size="md" loading={query.isFetching}><Search className="h-4 w-4" aria-hidden /> {t("search")}</Button>
        </form>
      </Card>

      <div aria-live="polite">
        {submitted === null && <InlineAlert tone="info">{t("intro")}</InlineAlert>}
        {query.isPending && submitted !== null && <Skeleton className="h-40" />}
        {query.isError && <ErrorState error={query.error} onRetry={() => query.refetch()} />}
        {query.isSuccess && query.data.agencies.length === 0 && (
          <div className="space-y-4">
            <EmptyState title={t("notFoundTitle")} body={t("notFoundBody")} />
          </div>
        )}
        {query.isSuccess && query.data.agencies.length > 0 && (
          <ul className="grid gap-4 sm:grid-cols-2">
            {query.data.agencies.map((a) => (
              <li key={a.reg_number}>
                <Card variant="bento" className="h-full">
                  <div className="flex items-start justify-between gap-2">
                    <h3 className="text-lg font-bold">{a.name}</h3>
                    {a.verified ? <Badge tone="success"><BadgeCheck className="h-3 w-3" aria-hidden /> {t("verified")}</Badge> : <Badge tone="warning">{t("notVerified")}</Badge>}
                  </div>
                  <dl className="mt-3 grid grid-cols-2 gap-2 text-sm">
                    <div><dt className="text-xs font-bold uppercase text-muted">{t("regNo")}</dt><dd className="font-mono">{a.reg_number}</dd></div>
                    <div><dt className="text-xs font-bold uppercase text-muted">{t("type")}</dt><dd>{a.type}</dd></div>
                    <div><dt className="text-xs font-bold uppercase text-muted">{t("district")}</dt><dd>{a.district}</dd></div>
                    <div><dt className="text-xs font-bold uppercase text-muted">{t("state")}</dt><dd>{a.state}</dd></div>
                  </dl>
                </Card>
              </li>
            ))}
          </ul>
        )}
        <p className="mt-4 text-xs text-muted">{t("footnote")}</p>
      </div>
    </div>
  );
}
