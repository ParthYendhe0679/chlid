"use client";

import * as React from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlarmClock, FileText, HeartHandshake, Lock, Stethoscope } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card, PageHead } from "@/components/ui/card";
import { Field, Textarea } from "@/components/ui/form";
import { Badge, EmptyState, ErrorState, InlineAlert, PageSkeleton } from "@/components/ui/feedback";
import { Sheet } from "@/components/ui/sheet";
import { useToast } from "@/components/ui/toast";
import { applicationsApi } from "@/lib/api/applications.api";
import { notReserveSchema } from "@/lib/validation/schemas";
import { useFieldError } from "@/lib/utils/use-form-error";
import { msUntil, splitDuration } from "@/lib/utils/format";
import { qk } from "@/features/case-tracker/queries";
import { useNow } from "@/features/case-tracker/use-ticker";
import { z } from "zod";

type Mode = null | "reserve" | "decline";

export function ReferralPage() {
  const t = useTranslations("referral");
  const fe = useFieldError();
  const toast = useToast();
  const qc = useQueryClient();
  const now = useNow();
  const [mode, setMode] = React.useState<Mode>(null);
  const [outcome, setOutcome] = React.useState<{ decision: "RESERVED" | "NOT_RESERVED"; status: string; warning?: string } | null>(null);

  const q = useQuery({ queryKey: qk.referral, queryFn: ({ signal }) => applicationsApi.activeReferral(signal) });

  const form = useForm<z.infer<typeof notReserveSchema>>({ resolver: zodResolver(notReserveSchema) });

  const decide = useMutation({
    mutationFn: (body: Parameters<typeof applicationsApi.decide>[1]) => applicationsApi.decide(q.data!.referral_id, body),
    onSuccess: (res, body) => {
      setMode(null);
      setOutcome({ decision: body.decision, status: res.status, warning: res.warning });
      toast({ tone: "success", title: t("decisionRecorded") });
      qc.invalidateQueries({ queryKey: qk.application });
      qc.invalidateQueries({ queryKey: qk.tracker });
      qc.invalidateQueries({ queryKey: qk.referral });
    },
    onError: (e) => {
      setMode(null);
      toast({ tone: "error", title: t("decisionFailed"), body: e.message });
    },
  });

  if (q.isPending) return <PageSkeleton />;
  if (q.isError) return <ErrorState error={q.error} onRetry={() => q.refetch()} />;

  if (outcome && !q.data) {
    return (
      <>
        <PageHead title={t("title")} />
        <Card variant="sticker">
          <h2 className="text-xl font-bold">{outcome.decision === "RESERVED" ? t("reservedTitle") : t("notReservedTitle")}</h2>
          <p className="mt-2 text-sm text-ink-soft">{t("serverStatus", { status: outcome.status })}</p>
          {outcome.warning && <InlineAlert tone="warning" className="mt-4">{outcome.warning}</InlineAlert>}
        </Card>
      </>
    );
  }

  const r = q.data;
  if (!r) {
    return (
      <>
        <PageHead kicker={t("kicker")} title={t("title")} subtitle={t("subtitle")} />
        <EmptyState icon={Lock} title={t("noneTitle")} body={t("noneBody")} />
        <Card variant="lavender" className="mt-6">
          <h2 className="text-lg font-bold">{t("lockTitle")}</h2>
          <p className="mt-2 text-sm text-ink-soft">{t("lockBody")}</p>
        </Card>
      </>
    );
  }

  const remaining = msUntil(r.expires_at, now);
  const d = splitDuration(remaining);
  const expired = remaining === 0;
  const last = r.referral_number >= 3;
  const pad = (n: number) => String(n).padStart(2, "0");

  return (
    <>
      <PageHead kicker={t("kicker")} title={t("title")} subtitle={t("subtitle")}>
        <Badge tone="purple">{t("number", { n: r.referral_number })}</Badge>
      </PageHead>

      <div className="grid gap-6 lg:grid-cols-[1.4fr_1fr]">
        <div className="space-y-6">
          <Card variant="sticker">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p className="text-xs font-bold uppercase tracking-wider text-muted">{t("caseCode")}</p>
                <p className="font-mono text-lg font-bold">{r.child.case_code}</p>
              </div>
              <dl className="flex gap-6 text-sm">
                <div><dt className="text-xs font-bold uppercase text-muted">{t("age")}</dt><dd className="font-semibold">{r.child.age_text}</dd></div>
                <div><dt className="text-xs font-bold uppercase text-muted">{t("genderLabel")}</dt><dd className="font-semibold">{t(`gender.${r.child.gender}`)}</dd></div>
                <div><dt className="text-xs font-bold uppercase text-muted">{t("special")}</dt><dd className="font-semibold">{r.child.is_special_needs ? t("yes") : t("no")}</dd></div>
              </dl>
            </div>
            <p className="mt-3 text-xs text-muted">{t("identityNote")}</p>
          </Card>

          <Card>
            <div className="flex items-center gap-2"><Stethoscope className="h-5 w-5 text-purple-deep" aria-hidden /><h2 className="text-lg font-bold">{t("summaryTitle")}</h2></div>
            <p className="mt-3 whitespace-pre-line text-[15px] leading-relaxed">{r.child.summary_approved}</p>
            <InlineAlert tone="info" className="mt-4">{t("summaryNote")}</InlineAlert>
          </Card>

          <Card variant="bento">
            <h2 className="text-lg font-bold">{t("reportsTitle")}</h2>
            <p className="mt-1 text-xs text-muted">{t("reportsNote")}</p>
            <div className="mt-3 flex flex-wrap gap-3">
              <a href={r.child.csr_url} target="_blank" rel="noopener noreferrer" className="inline-flex h-11 items-center gap-2 rounded-full bg-white px-5 text-sm font-semibold shadow-clay-sm hover:-translate-y-0.5"><FileText className="h-4 w-4" aria-hidden /> {t("csr")}</a>
              <a href={r.child.mer_url} target="_blank" rel="noopener noreferrer" className="inline-flex h-11 items-center gap-2 rounded-full bg-white px-5 text-sm font-semibold shadow-clay-sm hover:-translate-y-0.5"><FileText className="h-4 w-4" aria-hidden /> {t("mer")}</a>
              <Button variant="ghost" size="sm" onClick={() => q.refetch()} loading={q.isFetching}>{t("refreshLinks")}</Button>
            </div>
          </Card>
        </div>

        <div className="space-y-6">
          <Card variant={expired ? "pink" : "sun"}>
            <div className="flex items-center gap-2"><AlarmClock className="h-5 w-5" aria-hidden /><h2 className="text-lg font-bold">{t("timerTitle")}</h2></div>
            <p className="tabular mt-3 font-display text-4xl font-extrabold" role="timer" aria-label={t("timerLabel", { h: d.h, m: d.m })}>
              {pad(d.h)}:{pad(d.m)}:{pad(d.s)}
            </p>
            <p className="mt-2 text-xs text-ink-soft">{expired ? t("expired") : t("timerNote")}</p>
          </Card>

          <InlineAlert tone={last ? "danger" : "warning"} title={t("warnTitle", { n: r.referral_number })}>
            {last ? t("warnLast") : t("warnBody")}
          </InlineAlert>

          <Card>
            <h2 className="text-lg font-bold">{t("decideTitle")}</h2>
            <div className="mt-4 flex flex-col gap-3">
              <Button variant="purple" size="lg" disabled={expired || decide.isPending || decide.isSuccess} onClick={() => setMode("reserve")}>
                <HeartHandshake className="h-5 w-5" aria-hidden /> {t("reserve")}
              </Button>
              <Button variant="outline" size="lg" disabled={expired || decide.isPending || decide.isSuccess} onClick={() => setMode("decline")}>{t("decline")}</Button>
            </div>
            <p className="mt-3 text-xs text-muted">{t("decideNote")}</p>
          </Card>
        </div>
      </div>

      <Sheet
        open={mode === "reserve"}
        onClose={() => !decide.isPending && setMode(null)}
        title={t("confirmReserveTitle")}
        footer={
          <>
            <Button variant="secondary" onClick={() => setMode(null)} disabled={decide.isPending}>{t("back")}</Button>
            <Button variant="purple" loading={decide.isPending} onClick={() => decide.mutate({ decision: "RESERVED" })}>{t("confirmReserve")}</Button>
          </>
        }
      >
        <p>{t("confirmReserveBody")}</p>
      </Sheet>

      <Sheet
        open={mode === "decline"}
        onClose={() => !decide.isPending && setMode(null)}
        title={t("confirmDeclineTitle")}
        footer={
          <>
            <Button variant="secondary" onClick={() => setMode(null)} disabled={decide.isPending}>{t("back")}</Button>
            <Button variant="danger" loading={decide.isPending} onClick={form.handleSubmit((v) => decide.mutate({ decision: "NOT_RESERVED", reason: v.reason }))}>{t("confirmDecline")}</Button>
          </>
        }
      >
        <InlineAlert tone={last ? "danger" : "warning"} className="mb-4">{last ? t("warnLast") : t("warnBody")}</InlineAlert>
        <Field label={t("reasonLabel")} htmlFor="reason" error={fe(form.formState.errors.reason)}>
          <Textarea id="reason" rows={4} aria-invalid={!!form.formState.errors.reason} {...form.register("reason")} />
        </Field>
      </Sheet>
    </>
  );
}
