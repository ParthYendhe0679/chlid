"use client";

import * as React from "react";
import { Controller, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card, PageHead } from "@/components/ui/card";
import { Chip, Field, Input, Select, Switch, Textarea } from "@/components/ui/form";
import { ErrorState, InlineAlert, PageSkeleton } from "@/components/ui/feedback";
import { useToast } from "@/components/ui/toast";
import { applicationsApi } from "@/lib/api/applications.api";
import { ageGroups, maritalStatuses, profileSchema, type ProfileForm as Values } from "@/lib/validation/schemas";
import { useFieldError } from "@/lib/utils/use-form-error";
import { INDIAN_STATES } from "@/lib/utils/states";
import { qk, useMyApplication } from "./queries";
import { WaitTimeExplainer } from "./wait-time-explainer";
import type { ApplicationMe } from "@/types";

const num = (v: unknown) => (v === "" || v === null || v === undefined ? undefined : Number(v));
const EDITABLE = new Set(["REGISTERED", "DOCS_CORRECTION"]);

function toDefaults(a: ApplicationMe | null): Partial<Values> {
  if (!a) return { marital_status: "MARRIED", existing_children: 0, pref_age_group: "0-2", pref_gender: "ANY", pref_states: [], open_special_needs: false, open_siblings: false };
  return {
    marital_status: a.marital_status, years_married: a.years_married ?? undefined, applicant_dob: a.applicant_dob, spouse_dob: a.spouse_dob ?? undefined,
    existing_children: a.existing_children ?? 0, annual_income_inr: a.annual_income_inr ?? undefined, state: a.state, district: a.district, address: a.address ?? undefined,
    pref_age_group: a.pref_age_group, pref_gender: a.pref_gender, pref_states: a.pref_states ?? [],
    open_special_needs: a.open_special_needs ?? false, open_siblings: a.open_siblings ?? false,
  };
}

export function ProfilePage() {
  const q = useMyApplication();
  const t = useTranslations("profile");
  if (q.isPending) return <PageSkeleton />;
  if (q.isError) return <ErrorState error={q.error} onRetry={() => q.refetch()} />;
  return (
    <>
      <PageHead kicker={t("kicker")} title={t("title")} subtitle={t("subtitle")} />
      <ProfileForm existing={q.data} />
    </>
  );
}

function ProfileForm({ existing }: { existing: ApplicationMe | null }) {
  const t = useTranslations("profile");
  const tu = useTranslations("eligibility");
  const fe = useFieldError();
  const toast = useToast();
  const qc = useQueryClient();
  const editable = !existing || EDITABLE.has(existing.status);

  const { register, handleSubmit, control, watch, formState: { errors, isDirty } } = useForm<Values>({
    resolver: zodResolver(profileSchema),
    defaultValues: toDefaults(existing),
  });
  const marital = watch("marital_status");
  const prefAge = watch("pref_age_group");
  const openSpecial = watch("open_special_needs");

  const save = useMutation({
    mutationFn: async (v: Values) => {
      const body = { ...v, ...(v.marital_status !== "MARRIED" ? { spouse_dob: undefined, years_married: undefined } : {}) };
      return existing ? applicationsApi.update(body) : applicationsApi.create(body);
    },
    onSuccess: () => {
      toast({ tone: "success", title: existing ? t("updated") : t("created") });
      qc.invalidateQueries({ queryKey: qk.application });
      qc.invalidateQueries({ queryKey: qk.tracker });
    },
    onError: (e) => toast({ tone: "error", title: t("saveFailed"), body: e.message }),
  });

  return (
    <div className="grid gap-6 lg:grid-cols-[1.3fr_1fr]">
      <Card>
        {!editable && <InlineAlert tone="info" className="mb-5">{t("locked")}</InlineAlert>}
        <form onSubmit={handleSubmit((v) => save.mutate(v))} noValidate className="space-y-6">
          <fieldset disabled={!editable || save.isPending} className="space-y-4">
            <legend className="mb-2 text-lg font-bold">{t("about")}</legend>
            <Field label={t("marital")} htmlFor="marital_status">
              <Select id="marital_status" {...register("marital_status")}>
                {maritalStatuses.map((m) => <option key={m} value={m}>{tu(`maritalOptions.${m}`)}</option>)}
              </Select>
            </Field>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label={t("dob")} htmlFor="applicant_dob" error={fe(errors.applicant_dob)}>
                <Input id="applicant_dob" type="date" aria-invalid={!!errors.applicant_dob} {...register("applicant_dob")} />
              </Field>
              {marital === "MARRIED" && (
                <Field label={t("spouseDob")} htmlFor="spouse_dob" error={fe(errors.spouse_dob)}>
                  <Input id="spouse_dob" type="date" aria-invalid={!!errors.spouse_dob} {...register("spouse_dob")} />
                </Field>
              )}
              {marital === "MARRIED" && (
                <Field label={t("yearsMarried")} htmlFor="years_married" error={fe(errors.years_married)}>
                  <Input id="years_married" type="number" step="0.5" min={0} {...register("years_married", { setValueAs: num })} />
                </Field>
              )}
              <Field label={t("existingChildren")} htmlFor="existing_children" error={fe(errors.existing_children)}>
                <Input id="existing_children" type="number" min={0} {...register("existing_children", { valueAsNumber: true })} />
              </Field>
              <Field label={t("income")} hint={t("incomeHint")} htmlFor="annual_income_inr" error={fe(errors.annual_income_inr)}>
                <Input id="annual_income_inr" type="number" min={0} {...register("annual_income_inr", { setValueAs: num })} />
              </Field>
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label={t("state")} htmlFor="state" error={fe(errors.state)}>
                <Select id="state" aria-invalid={!!errors.state} {...register("state")}>
                  <option value="">{t("choose")}</option>
                  {INDIAN_STATES.map((s) => <option key={s} value={s}>{s}</option>)}
                </Select>
              </Field>
              <Field label={t("district")} htmlFor="district" error={fe(errors.district)}>
                <Input id="district" aria-invalid={!!errors.district} {...register("district")} />
              </Field>
            </div>
            <Field label={t("address")} hint={t("addressHint")} htmlFor="address">
              <Textarea id="address" rows={3} {...register("address")} />
            </Field>
          </fieldset>

          <fieldset disabled={!editable || save.isPending} className="space-y-4">
            <legend className="mb-2 text-lg font-bold">{t("preferences")}</legend>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label={t("prefAge")} htmlFor="pref_age_group">
                <Select id="pref_age_group" {...register("pref_age_group")}>
                  {ageGroups.map((g) => <option key={g} value={g}>{t("ageOption", { range: g })}</option>)}
                </Select>
              </Field>
              <Field label={t("prefGender")} htmlFor="pref_gender">
                <Select id="pref_gender" {...register("pref_gender")}>
                  {(["ANY", "MALE", "FEMALE"] as const).map((g) => <option key={g} value={g}>{t(`gender.${g}`)}</option>)}
                </Select>
              </Field>
            </div>
            <div>
              <p className="mb-2 text-[13px] font-bold text-ink-soft">{t("prefStates")}</p>
              <p className="mb-2 text-xs text-muted">{t("prefStatesHint")}</p>
              <Controller
                control={control}
                name="pref_states"
                render={({ field }) => (
                  <div className="flex max-h-40 flex-wrap gap-2 overflow-y-auto p-1" role="group" aria-label={t("prefStates")}>
                    {INDIAN_STATES.map((s) => (
                      <Chip key={s} size="sm" selected={field.value.includes(s)} onClick={() => field.onChange(field.value.includes(s) ? field.value.filter((x) => x !== s) : [...field.value, s])}>
                        {s}
                      </Chip>
                    ))}
                  </div>
                )}
              />
            </div>
            <Controller control={control} name="open_special_needs" render={({ field }) => <Switch id="open_special_needs" label={t("openSpecial")} checked={field.value} onChange={field.onChange} />} />
            <Controller control={control} name="open_siblings" render={({ field }) => <Switch id="open_siblings" label={t("openSiblings")} checked={field.value} onChange={field.onChange} />} />
          </fieldset>

          {editable && (
            <div className="flex items-center gap-3">
              <Button type="submit" size="lg" loading={save.isPending} disabled={!!existing && !isDirty}>{existing ? t("save") : t("create")}</Button>
              {existing && !isDirty && <span className="text-xs text-muted">{t("noChanges")}</span>}
            </div>
          )}
        </form>
      </Card>

      <aside className="space-y-4">
        <WaitTimeExplainer ageGroup={prefAge} openSpecialNeeds={openSpecial} />
        <InlineAlert tone="info">{t("privacy")}</InlineAlert>
      </aside>
    </div>
  );
}
