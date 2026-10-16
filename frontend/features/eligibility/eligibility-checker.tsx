"use client";

import * as React from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation } from "@tanstack/react-query";
import { CheckCircle2, XCircle } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Field, Input, Select } from "@/components/ui/form";
import { Badge, ErrorState, InlineAlert } from "@/components/ui/feedback";
import { publicApi } from "@/lib/api/public.api";
import { eligibilitySchema, maritalStatuses, type EligibilityForm } from "@/lib/validation/schemas";
import { useFieldError } from "@/lib/utils/use-form-error";
import type { AgeGroup } from "@/types";

const numberOrUndef = (v: unknown) => (v === "" || v === null || v === undefined ? undefined : Number(v));

export function EligibilityChecker() {
  const t = useTranslations("eligibility");
  const fe = useFieldError();
  const { register, handleSubmit, watch, formState: { errors } } = useForm<EligibilityForm>({
    resolver: zodResolver(eligibilitySchema),
    defaultValues: { marital_status: "MARRIED", existing_children: 0 },
  });
  const marital = watch("marital_status");
  const mutation = useMutation({ mutationFn: publicApi.eligibility });

  const onSubmit = handleSubmit((v) => {
    mutation.mutate({
      marital_status: v.marital_status,
      applicant_dob: v.applicant_dob,
      existing_children: v.existing_children,
      ...(v.marital_status === "MARRIED" ? { spouse_dob: v.spouse_dob, years_married: v.years_married } : {}),
    });
  });

  const result = mutation.data;
  const groups: AgeGroup[] = ["0-2", "2-4", "4-8", "8-18"];

  return (
    <div className="grid gap-6 lg:grid-cols-[1fr_1.1fr]">
      <Card>
        <form onSubmit={onSubmit} noValidate className="space-y-4" aria-describedby="elig-note">
          <Field label={t("marital")} htmlFor="marital_status">
            <Select id="marital_status" {...register("marital_status")}>
              {maritalStatuses.map((m) => <option key={m} value={m}>{t(`maritalOptions.${m}`)}</option>)}
            </Select>
          </Field>
          <Field label={marital === "MARRIED" ? t("dobYou") : t("dob")} htmlFor="applicant_dob" error={fe(errors.applicant_dob)}>
            <Input id="applicant_dob" type="date" aria-invalid={!!errors.applicant_dob} {...register("applicant_dob")} />
          </Field>
          {marital === "MARRIED" && (
            <>
              <Field label={t("dobSpouse")} htmlFor="spouse_dob" error={fe(errors.spouse_dob)}>
                <Input id="spouse_dob" type="date" aria-invalid={!!errors.spouse_dob} {...register("spouse_dob")} />
              </Field>
              <Field label={t("yearsMarried")} hint={t("yearsMarriedHint")} htmlFor="years_married" error={fe(errors.years_married)}>
                <Input id="years_married" type="number" step="0.5" min={0} aria-invalid={!!errors.years_married} {...register("years_married", { setValueAs: numberOrUndef })} />
              </Field>
            </>
          )}
          <Field label={t("existingChildren")} htmlFor="existing_children" error={fe(errors.existing_children)}>
            <Input id="existing_children" type="number" min={0} aria-invalid={!!errors.existing_children} {...register("existing_children", { valueAsNumber: true })} />
          </Field>
          <Button type="submit" size="lg" className="w-full" loading={mutation.isPending}>{t("check")}</Button>
          <p id="elig-note" className="text-xs text-muted">{t("privacyNote")}</p>
        </form>
      </Card>

      <div aria-live="polite" className="space-y-4">
        {mutation.isError && <ErrorState error={mutation.error} onRetry={() => onSubmit()} />}
        {!result && !mutation.isError && (
          <Card variant="lavender">
            <h2 className="text-lg font-bold">{t("howTitle")}</h2>
            <p className="mt-2 text-sm text-ink-soft">{t("howBody")}</p>
          </Card>
        )}
        {result && (
          <Card variant="sticker">
            <div className="flex items-center gap-3">
              {result.eligible ? <CheckCircle2 className="h-8 w-8 text-success" aria-hidden /> : <XCircle className="h-8 w-8 text-danger" aria-hidden />}
              <h2 className="text-xl font-bold">{result.eligible ? t("eligible") : t("notEligible")}</h2>
            </div>
            <h3 className="mt-5 text-sm font-bold uppercase tracking-wider text-muted">{t("ageGroups")}</h3>
            <ul className="mt-2 grid grid-cols-2 gap-2 sm:grid-cols-4">
              {groups.map((g) => {
                const ok = result.allowed_age_groups.includes(g);
                return (
                  <li key={g} className={`rounded-2xl p-3 text-center ${ok ? "bg-success-soft" : "bg-ink/5"}`}>
                    <p className="tabular font-display text-lg font-bold">{g}</p>
                    <Badge tone={ok ? "success" : "neutral"}>{ok ? t("allowed") : t("notAllowed")}</Badge>
                  </li>
                );
              })}
            </ul>
            {result.reasons.length > 0 && (
              <>
                <h3 className="mt-5 text-sm font-bold uppercase tracking-wider text-muted">{t("reasons")}</h3>
                <ul className="mt-2 space-y-2">
                  {result.reasons.map((r) => (
                    <li key={r.code + r.message} className="rounded-2xl bg-canvas p-3 text-sm">{r.message}</li>
                  ))}
                </ul>
              </>
            )}
            <InlineAlert tone="info" className="mt-5">{t("disclaimer")}</InlineAlert>
          </Card>
        )}
      </div>
    </div>
  );
}
