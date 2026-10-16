"use client";

import * as React from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Gavel } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Field, Input, Select } from "@/components/ui/form";
import { InlineAlert } from "@/components/ui/feedback";
import { useToast } from "@/components/ui/toast";
import { agencyApi } from "@/lib/api/agency.api";
import { ApiError } from "@/lib/api/errors";
import { legalEventSchema, type LegalEventForm } from "@/lib/validation/schemas";
import { useFieldError } from "@/lib/utils/use-form-error";
import { formatDate } from "@/lib/utils/format";
import type { AgencyCaseDetail } from "@/types";

const TYPES = ["MATCHED", "FOSTER_CARE", "DM_FILED", "DM_ORDER", "BIRTH_CERT"] as const;

export function LegalTimeline({ detail }: { detail: AgencyCaseDetail }) {
  const t = useTranslations("legal");
  const fe = useFieldError();
  const locale = useLocale();
  const toast = useToast();
  const qc = useQueryClient();
  const { register, handleSubmit, reset, formState: { errors } } = useForm<LegalEventForm>({
    resolver: zodResolver(legalEventSchema),
    defaultValues: { event_type: "MATCHED" },
  });
  const m = useMutation({
    mutationFn: (v: LegalEventForm) => agencyApi.recordEvent(detail.application_id, { ...v, reference_no: v.reference_no || undefined }),
    onSuccess: (r) => {
      toast({ tone: "success", title: t("recorded"), body: t("newStatus", { status: r.status }) });
      reset({ event_type: "MATCHED", event_date: "", reference_no: "" });
      qc.invalidateQueries({ queryKey: ["agency", "case", detail.application_id] });
    },
    onError: (e) => toast({ tone: "error", title: e instanceof ApiError && e.code === "INVALID_TRANSITION" ? t("invalidTransition") : t("failed"), body: e.message }),
  });

  return (
    <div className="grid gap-6 lg:grid-cols-[1.2fr_1fr]">
      <Card>
        <h3 className="text-lg font-bold">{t("history")}</h3>
        {detail.timeline.length === 0 ? <p className="mt-3 text-sm text-muted">{t("none")}</p> : (
          <ol className="mt-4 space-y-3">
            {detail.timeline.map((e, i) => (
              <li key={i} className="flex items-center justify-between gap-3 rounded-2xl bg-canvas p-3 text-sm">
                <span className="flex items-center gap-2 font-semibold"><Gavel className="h-4 w-4 text-purple-deep" aria-hidden />{t(`types.${e.event_type}`)}</span>
                <span className="tabular text-xs text-muted">{formatDate(e.event_date, locale)}{e.reference_no ? ` | ${e.reference_no}` : ""}</span>
              </li>
            ))}
          </ol>
        )}
      </Card>
      <Card variant="lavender">
        <h3 className="text-lg font-bold">{t("record")}</h3>
        <p className="mt-1 text-xs text-ink-soft">{t("autoDeadline")}</p>
        <form onSubmit={handleSubmit((v) => m.mutate(v))} noValidate className="mt-4 space-y-4">
          <Field label={t("type")} htmlFor="event_type"><Select id="event_type" {...register("event_type")}>{TYPES.map((x) => <option key={x} value={x}>{t(`types.${x}`)}</option>)}</Select></Field>
          <Field label={t("date")} htmlFor="event_date" error={fe(errors.event_date)}><Input id="event_date" type="date" aria-invalid={!!errors.event_date} {...register("event_date")} /></Field>
          <Field label={t("reference")} htmlFor="reference_no"><Input id="reference_no" {...register("reference_no")} /></Field>
          <Button type="submit" loading={m.isPending} className="w-full">{t("save")}</Button>
          <InlineAlert tone="info">{t("serverDecides")}</InlineAlert>
        </form>
      </Card>
    </div>
  );
}
