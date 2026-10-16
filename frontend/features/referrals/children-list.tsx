"use client";

import * as React from "react";
import Link from "next/link";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Baby, ChevronRight, Plus } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card, PageHead } from "@/components/ui/card";
import { Field, Input, Select, Switch } from "@/components/ui/form";
import { Badge, EmptyState, ErrorState, InlineAlert, Skeleton } from "@/components/ui/feedback";
import { Sheet } from "@/components/ui/sheet";
import { useToast } from "@/components/ui/toast";
import { agencyApi } from "@/lib/api/agency.api";
import { ApiError } from "@/lib/api/errors";
import { addChildSchema, type AddChildForm } from "@/lib/validation/schemas";
import { useFieldError } from "@/lib/utils/use-form-error";

const tone = { IN_CARE: "neutral", LEGALLY_FREE: "info", READY: "success", REFERRED: "purple", RESERVED: "purple", PLACED: "success" } as const;

export function ChildrenList() {
  const t = useTranslations("children");
  const [open, setOpen] = React.useState(false);
  const q = useQuery({ queryKey: ["agency", "children"], queryFn: ({ signal }) => agencyApi.children(signal), retry: false });
  // GET /agency/children is a PROPOSED endpoint (not in guide 15.6): surface a precise message when it is missing.
  const missingEndpoint = q.isError && q.error instanceof ApiError && (q.error.status === 404 || q.error.status === 405);

  return (
    <>
      <PageHead kicker={t("kicker")} title={t("title")} subtitle={t("subtitle")}>
        <Button onClick={() => setOpen(true)}><Plus className="h-4 w-4" aria-hidden /> {t("add")}</Button>
      </PageHead>
      <InlineAlert tone="info" className="mb-5">{t("privacy")}</InlineAlert>

      {q.isPending && <div className="grid gap-4 sm:grid-cols-2"><Skeleton className="h-24" /><Skeleton className="h-24" /></div>}
      {missingEndpoint && <InlineAlert tone="warning" title={t("endpointMissingTitle")}>{t("endpointMissingBody")}</InlineAlert>}
      {q.isError && !missingEndpoint && <ErrorState error={q.error} onRetry={() => q.refetch()} />}
      {q.isSuccess && q.data.length === 0 && <EmptyState icon={Baby} title={t("emptyTitle")} body={t("emptyBody")} />}
      {q.isSuccess && q.data.length > 0 && (
        <ul className="grid gap-4 sm:grid-cols-2">
          {q.data.map((c) => (
            <li key={c.id}>
              <Link href={`/agency/children/${c.id}`} className="block">
                <Card variant="bento" interactive className="flex items-center justify-between gap-3">
                  <div>
                    <p className="font-mono text-base font-bold">{c.case_code}</p>
                    <p className="mt-1 text-xs text-muted">{c.age_text} | {t(`gender.${c.gender}`)}{c.is_special_needs ? ` | ${t("special")}` : ""}</p>
                    <div className="mt-2"><Badge tone={tone[c.status]}>{t(`status.${c.status}`)}</Badge></div>
                  </div>
                  <ChevronRight className="h-5 w-5 text-muted" aria-hidden />
                </Card>
              </Link>
            </li>
          ))}
        </ul>
      )}
      <AddChildSheet open={open} onClose={() => setOpen(false)} />
    </>
  );
}

function AddChildSheet({ open, onClose }: { open: boolean; onClose: () => void }) {
  const t = useTranslations("children");
  const fe = useFieldError();
  const toast = useToast();
  const qc = useQueryClient();
  const { register, handleSubmit, control, reset, formState: { errors } } = useForm<AddChildForm>({
    resolver: zodResolver(addChildSchema),
    defaultValues: { gender: "FEMALE", is_special_needs: false },
  });
  const m = useMutation({
    mutationFn: agencyApi.addChild,
    onSuccess: (r) => {
      toast({ tone: "success", title: t("added"), body: t("caseCode", { code: r.case_code }) });
      reset();
      onClose();
      qc.invalidateQueries({ queryKey: ["agency", "children"] });
    },
    onError: (e) => toast({ tone: "error", title: t("addFailed"), body: e.message }),
  });

  return (
    <Sheet
      open={open}
      onClose={() => !m.isPending && onClose()}
      title={t("addTitle")}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={m.isPending}>{t("cancel")}</Button>
          <Button loading={m.isPending} onClick={handleSubmit((v) => m.mutate(v))}>{t("addSubmit")}</Button>
        </>
      }
    >
      <form className="space-y-4" noValidate onSubmit={(e) => e.preventDefault()} autoComplete="off">
        <InlineAlert tone="warning">{t("nameWarning")}</InlineAlert>
        <Field label={t("name")} htmlFor="c-name" error={fe(errors.name)}><Input id="c-name" autoComplete="off" aria-invalid={!!errors.name} {...register("name")} /></Field>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t("dob")} htmlFor="c-dob" error={fe(errors.dob)}><Input id="c-dob" type="date" aria-invalid={!!errors.dob} {...register("dob")} /></Field>
          <Field label={t("genderLabel")} htmlFor="c-gender"><Select id="c-gender" {...register("gender")}><option value="FEMALE">{t("gender.FEMALE")}</option><option value="MALE">{t("gender.MALE")}</option></Select></Field>
          <Field label={t("admittedOn")} htmlFor="c-adm" error={fe(errors.admitted_on)}><Input id="c-adm" type="date" {...register("admitted_on")} /></Field>
          <Field label={t("lfaDate")} htmlFor="c-lfad" error={fe(errors.lfa_date)}><Input id="c-lfad" type="date" {...register("lfa_date")} /></Field>
        </div>
        <Field label={t("cwcNumber")} htmlFor="c-cwc" error={fe(errors.cwc_lfa_number)}><Input id="c-cwc" {...register("cwc_lfa_number")} /></Field>
        <Controller control={control} name="is_special_needs" render={({ field }) => <Switch id="c-special" label={t("specialNeeds")} checked={field.value} onChange={field.onChange} />} />
      </form>
    </Sheet>
  );
}
