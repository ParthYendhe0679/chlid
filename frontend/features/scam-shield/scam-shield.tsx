"use client";

import * as React from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation } from "@tanstack/react-query";
import { Flag, Phone } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Field, Textarea } from "@/components/ui/form";
import { AiLabel, Badge, ErrorState, InlineAlert } from "@/components/ui/feedback";
import { useToast } from "@/components/ui/toast";
import { publicApi } from "@/lib/api/public.api";
import { scamSchema } from "@/lib/validation/schemas";
import { useFieldError } from "@/lib/utils/use-form-error";
import type { Lang, ScamCheckResponse } from "@/types";
import { z } from "zod";

type Values = z.infer<typeof scamSchema>;
const verdictTone = { LIKELY_SCAM: "danger", SUSPICIOUS: "warning", LOOKS_SAFE: "success" } as const;

export function ScamShield() {
  const t = useTranslations("scam");
  const locale = useLocale() as Lang;
  const fe = useFieldError();
  const toast = useToast();
  const { register, handleSubmit, formState: { errors } } = useForm<Values>({ resolver: zodResolver(scamSchema) });
  const check = useMutation({ mutationFn: (text: string) => publicApi.scamCheck(text, locale) });
  const report = useMutation({
    mutationFn: (token: string) => publicApi.scamReport(token),
    onSuccess: () => toast({ tone: "success", title: t("reported") }),
    onError: (e) => toast({ tone: "error", title: t("reportFailed"), body: e.message }),
  });
  const result: ScamCheckResponse | undefined = check.data;

  return (
    <div className="grid gap-6 lg:grid-cols-[1fr_1fr]">
      <Card>
        <form onSubmit={handleSubmit((v) => check.mutate(v.text))} noValidate className="space-y-4">
          <Field label={t("label")} hint={t("hint")} htmlFor="text" error={fe(errors.text)}>
            <Textarea id="text" rows={8} aria-invalid={!!errors.text} {...register("text")} />
          </Field>
          <Button type="submit" size="lg" className="w-full" loading={check.isPending}>{t("check")}</Button>
          <p className="text-xs text-muted">{t("privacy")}</p>
        </form>
      </Card>

      <div className="space-y-4" aria-live="polite">
        {check.isError && <ErrorState error={check.error} onRetry={() => check.reset()} />}
        {!result && !check.isError && (
          <Card variant="coral">
            <h2 className="text-lg font-bold">{t("whyTitle")}</h2>
            <p className="mt-2 text-sm text-ink-soft">{t("whyBody")}</p>
          </Card>
        )}
        {result && (
          <Card variant="sticker">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <Badge tone={verdictTone[result.verdict]}>{t(`verdict.${result.verdict}`)}</Badge>
              <AiLabel />
            </div>
            <p className="mt-3 text-xs font-semibold text-muted">{t("notConfirmed")}</p>
            <h3 className="mt-4 text-sm font-bold uppercase tracking-wider text-muted">{t("signs")}</h3>
            <ul className="mt-2 space-y-2">
              {result.reasons.map((r) => <li key={r} className="rounded-2xl bg-canvas p-3 text-sm">{r}</li>)}
            </ul>
            <InlineAlert tone="info" className="mt-4" title={t("legalTitle")}>{result.advice}</InlineAlert>
            <div className="mt-4 flex flex-wrap gap-2">
              <Button variant="outline" size="sm" loading={report.isPending} disabled={report.isSuccess} onClick={() => report.mutate(result.report_token)}>
                <Flag className="h-4 w-4" aria-hidden /> {report.isSuccess ? t("reported") : t("report")}
              </Button>
            </div>
          </Card>
        )}
        <Card variant="mint">
          <div className="flex items-start gap-3">
            <Phone className="mt-1 h-5 w-5 shrink-0 text-mint-deep" aria-hidden />
            <div>
              <h2 className="font-bold">{t("reportTitle")}</h2>
              <p className="mt-1 text-sm text-ink-soft">{t("reportBody")}</p>
            </div>
          </div>
        </Card>
      </div>
    </div>
  );
}
