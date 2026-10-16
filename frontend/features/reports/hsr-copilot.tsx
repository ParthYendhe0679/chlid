"use client";

import * as React from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { FilePenLine, PenLine, Save, Sparkles, X } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Field, Input, Select, Textarea } from "@/components/ui/form";
import { AiLabel, Badge, InlineAlert, ProgressBar } from "@/components/ui/feedback";
import { Sheet } from "@/components/ui/sheet";
import { useToast } from "@/components/ui/toast";
import { aiApi } from "@/lib/api/ai.api";
import { agencyApi } from "@/lib/api/agency.api";
import { ApiError } from "@/lib/api/errors";
import { hsrNotesSchema } from "@/lib/validation/schemas";
import { useFieldError } from "@/lib/utils/use-form-error";
import { formatDateTime } from "@/lib/utils/format";
import type { AgencyCaseDetail, HsrSections } from "@/types";
import { z } from "zod";

export const HSR_SECTIONS: (keyof HsrSections)[] = [
  "family_background", "home_environment", "health", "finances", "motivation_readiness", "parenting_plan", "support_system", "observations",
];
type Recommendation = "SUITABLE" | "NOT_SUITABLE" | "NEEDS_MORE_INFO";
const empty = (): HsrSections => Object.fromEntries(HSR_SECTIONS.map((k) => [k, ""])) as unknown as HsrSections;

export function HsrCopilot({ detail }: { detail: AgencyCaseDetail }) {
  const t = useTranslations("hsr");
  const fe = useFieldError();
  const locale = useLocale();
  const toast = useToast();
  const qc = useQueryClient();
  const hsr = detail.hsr;
  const signed = !!hsr?.signed_at;
  const caseId = detail.application_id;

  const [sections, setSections] = React.useState<HsrSections>(() => ({ ...empty(), ...(hsr?.final_sections ?? {}) }));
  const [saved, setSaved] = React.useState<HsrSections>(sections);
  const [aiDraft, setAiDraft] = React.useState<HsrSections | null>(hsr?.ai_draft?.sections ?? null);
  const [missing, setMissing] = React.useState<string[]>(hsr?.missing_items ?? []);
  const [score, setScore] = React.useState<number | null>(hsr?.completeness_score ?? null);
  const [aiRunId, setAiRunId] = React.useState<string | null>(hsr?.ai_run_id ?? hsr?.ai_draft?.ai_run_id ?? null);
  const [model, setModel] = React.useState<string | null>(hsr?.ai_draft?.model ?? null);
  const [visitDate, setVisitDate] = React.useState(hsr?.visit_date ?? "");
  const [recommendation, setRecommendation] = React.useState<Recommendation | "">(hsr?.recommendation ?? "");
  const [savedAt, setSavedAt] = React.useState<string | null>(null);
  const [signOpen, setSignOpen] = React.useState(false);
  const abortRef = React.useRef<AbortController | null>(null);

  const dirty = HSR_SECTIONS.some((k) => sections[k] !== saved[k]);
  const hasContent = HSR_SECTIONS.some((k) => sections[k].trim());

  // Warn before leaving with unsaved edits.
  React.useEffect(() => {
    if (!dirty) return;
    const h = (e: BeforeUnloadEvent) => { e.preventDefault(); };
    window.addEventListener("beforeunload", h);
    return () => window.removeEventListener("beforeunload", h);
  }, [dirty]);

  const notes = useForm<z.infer<typeof hsrNotesSchema>>({
    resolver: zodResolver(hsrNotesSchema),
    defaultValues: { visit_date: hsr?.visit_date ?? "", raw_notes: hsr?.raw_notes ?? "" },
  });

  const draft = useMutation({
    mutationFn: async (v: z.infer<typeof hsrNotesSchema>) => {
      abortRef.current = new AbortController();
      return aiApi.hsrDraft({ application_id: caseId, ...v }, abortRef.current.signal);
    },
    onSuccess: (res, v) => {
      // Never overwrite human edits silently.
      if (dirty && !window.confirm(t("overwriteConfirm"))) return;
      const next = { ...empty(), ...res.sections };
      setSections(next);
      setAiDraft(next);
      setMissing(res.missing_items);
      setScore(res.completeness_score);
      setAiRunId(res.ai_run_id);
      setModel(res.model ?? null);
      setVisitDate(v.visit_date);
      toast({ tone: "ai", title: t("draftReady"), body: t("draftReadyBody") });
    },
    onError: (e) => {
      if (e instanceof ApiError && e.code === "ABORTED") return;
      toast({ tone: "error", title: t("draftFailed"), body: e instanceof ApiError && e.code === "AI_UNAVAILABLE" ? t("aiDown") : e.message });
    },
  });

  const save = useMutation({
    mutationFn: () => agencyApi.saveHsr(caseId, { visit_date: visitDate, final_sections: sections, ...(recommendation ? { recommendation } : {}) }),
    onSuccess: () => {
      setSaved(sections);
      setSavedAt(new Date().toISOString());
      toast({ tone: "success", title: t("saved") });
      qc.invalidateQueries({ queryKey: ["agency", "case", caseId] });
    },
    onError: (e) => toast({ tone: "error", title: t("saveFailed"), body: e.message }),
  });

  const sign = useMutation({
    mutationFn: () => agencyApi.signHsr(caseId, aiRunId!),
    onSuccess: (r) => {
      setSignOpen(false);
      toast({ tone: "success", title: t("signed"), body: t("signedBody", { status: r.status }) });
      qc.invalidateQueries({ queryKey: ["agency", "case", caseId] });
      qc.invalidateQueries({ queryKey: ["agency", "cases"] });
    },
    onError: (e) => { setSignOpen(false); toast({ tone: "error", title: t("signFailed"), body: e.message }); },
  });

  const emptyRequired = HSR_SECTIONS.filter((k) => !sections[k].trim());
  // Missing items reported by the draft disappear as soon as the social worker fills them in.
  const stillMissing = missing.filter((m) => !(HSR_SECTIONS as string[]).includes(m) || !sections[m as keyof HsrSections].trim());
  const canSign = !signed && !dirty && !!recommendation && !!aiRunId && hasContent && !!visitDate;

  if (detail.status !== "HSR_IN_PROGRESS" && !hsr) {
    return <InlineAlert tone="info">{t("notReady", { status: detail.status })}</InlineAlert>;
  }

  return (
    <div className="space-y-6">
      {signed && <InlineAlert tone="success" title={t("alreadySigned")}>{t("signedAt", { date: formatDateTime(hsr?.signed_at, locale) })}</InlineAlert>}

      {!signed && (
        <Card variant="lavender">
          <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
            <h3 className="flex items-center gap-2 text-lg font-bold"><Sparkles className="h-5 w-5" aria-hidden /> {t("notesTitle")}</h3>
            <AiLabel />
          </div>
          <form onSubmit={notes.handleSubmit((v) => draft.mutate(v))} noValidate className="space-y-4">
            <Field label={t("visitDate")} htmlFor="visit_date" error={fe(notes.formState.errors.visit_date)} className="max-w-xs">
              <Input id="visit_date" type="date" aria-invalid={!!notes.formState.errors.visit_date} {...notes.register("visit_date")} />
            </Field>
            <Field label={t("rawNotes")} hint={t("rawNotesHint")} htmlFor="raw_notes" error={fe(notes.formState.errors.raw_notes)}>
              <Textarea id="raw_notes" rows={8} aria-invalid={!!notes.formState.errors.raw_notes} {...notes.register("raw_notes")} />
            </Field>
            <div className="flex flex-wrap items-center gap-3">
              <Button type="submit" loading={draft.isPending}><Sparkles className="h-4 w-4" aria-hidden /> {t("draftWithAi")}</Button>
              {draft.isPending && <Button variant="outline" onClick={() => abortRef.current?.abort()}><X className="h-4 w-4" aria-hidden /> {t("cancel")}</Button>}
            </div>
            {draft.isPending && (
              <div className="space-y-1" role="status">
                <ProgressBar indeterminate label={t("drafting")} />
                <p className="text-xs text-muted">{t("draftingNote")}</p>
              </div>
            )}
            <p className="text-xs text-muted">{t("noFabrication")}</p>
          </form>
        </Card>
      )}

      <Card>
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <h3 className="flex items-center gap-2 text-lg font-bold"><FilePenLine className="h-5 w-5" aria-hidden /> {t("reportTitle")}</h3>
          <div className="flex flex-wrap items-center gap-2">
            {aiDraft && <Badge tone="purple">{t("aiDraftBadge")}</Badge>}
            {aiDraft && model && <Badge tone="neutral" dot={false}><span className="font-mono">{model}</span></Badge>}
            {dirty && <Badge tone="warning">{t("unsaved")}</Badge>}
            {!dirty && savedAt && <Badge tone="success">{t("savedAt", { date: formatDateTime(savedAt, locale) })}</Badge>}
            {score !== null && <Badge tone={score >= 80 ? "success" : "warning"}>{t("completeness", { score })}</Badge>}
          </div>
        </div>

        {!signed && stillMissing.length > 0 && (
          <InlineAlert tone="warning" className="mb-4" title={t("missingTitle")}>
            <ul className="mt-1 list-disc pl-5">{stillMissing.map((m) => <li key={m}>{t.has(`sections.${m}`) ? t(`sections.${m}`) : m}</li>)}</ul>
            <p className="mt-1 text-xs">{t("missingNote")}</p>
          </InlineAlert>
        )}
        {!hasContent && <InlineAlert tone="info" className="mb-4">{t("emptyEditor")}</InlineAlert>}

        <div className="space-y-5">
          {HSR_SECTIONS.map((k) => {
            const flagged = missing.includes(k) || (hasContent && !sections[k].trim());
            return (
              <div key={k}>
                <Field label={t(`sections.${k}`)} htmlFor={`sec-${k}`} hint={flagged ? t("flagged") : undefined}>
                  <Textarea
                    id={`sec-${k}`}
                    rows={4}
                    value={sections[k]}
                    disabled={signed}
                    aria-invalid={flagged}
                    onChange={(e) => setSections((s) => ({ ...s, [k]: e.target.value }))}
                    className={flagged ? "border-warning" : undefined}
                  />
                </Field>
                {aiDraft && aiDraft[k] !== sections[k] && aiDraft[k] && (
                  <details className="mt-1 text-xs">
                    <summary className="cursor-pointer font-semibold text-purple-deep">{t("showAiOriginal")}</summary>
                    <p className="mt-1 whitespace-pre-wrap rounded-xl bg-lavender-soft p-2">{aiDraft[k]}</p>
                  </details>
                )}
              </div>
            );
          })}
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label={t("visitDate")} htmlFor="final_visit_date">
              <Input id="final_visit_date" type="date" value={visitDate} disabled={signed} onChange={(e) => setVisitDate(e.target.value)} />
            </Field>
            <Field label={t("recommendation")} htmlFor="recommendation">
              <Select id="recommendation" value={recommendation} disabled={signed} onChange={(e) => setRecommendation(e.target.value as Recommendation | "")}>
                <option value="">{t("choose")}</option>
                {(["SUITABLE", "NOT_SUITABLE", "NEEDS_MORE_INFO"] as const).map((r) => <option key={r} value={r}>{t(`rec.${r}`)}</option>)}
              </Select>
            </Field>
          </div>
        </div>

        {!signed && (
          <div className="mt-6 flex flex-wrap gap-3">
            <Button variant="secondary" onClick={() => save.mutate()} loading={save.isPending} disabled={!dirty && !!savedAt || !hasContent || !visitDate}><Save className="h-4 w-4" aria-hidden /> {t("saveDraft")}</Button>
            <Button onClick={() => setSignOpen(true)} disabled={!canSign}><PenLine className="h-4 w-4" aria-hidden /> {t("signSubmit")}</Button>
          </div>
        )}
        {!signed && !canSign && <p className="mt-2 text-xs text-muted">{!aiRunId ? t("needAiRun") : dirty ? t("saveFirst") : !recommendation ? t("needRec") : ""}</p>}
      </Card>

      <Sheet
        open={signOpen}
        onClose={() => !sign.isPending && setSignOpen(false)}
        title={t("signTitle")}
        footer={
          <>
            <Button variant="secondary" onClick={() => setSignOpen(false)} disabled={sign.isPending}>{t("cancel")}</Button>
            <Button onClick={() => sign.mutate()} loading={sign.isPending}><PenLine className="h-4 w-4" aria-hidden /> {t("confirmSign")}</Button>
          </>
        }
      >
        <p>{t("signBody")}</p>
        {recommendation && <p className="mt-2 font-semibold">{t("recommendation")}: {t(`rec.${recommendation}`)}</p>}
        {emptyRequired.length > 0 && (
          <InlineAlert tone="warning" className="mt-3">{t("signEmpty", { count: emptyRequired.length })}</InlineAlert>
        )}
      </Sheet>
    </div>
  );
}
