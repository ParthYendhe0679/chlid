"use client";

import * as React from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, Check, Circle, FileUp, Send, Sparkles, UserSearch } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card, PageHead } from "@/components/ui/card";
import { Field, Textarea } from "@/components/ui/form";
import { AiLabel, Badge, ErrorState, InlineAlert, PageSkeleton, ProgressBar } from "@/components/ui/feedback";
import { Sheet } from "@/components/ui/sheet";
import { useToast } from "@/components/ui/toast";
import { aiApi } from "@/lib/api/ai.api";
import { agencyApi } from "@/lib/api/agency.api";
import { ApiError } from "@/lib/api/errors";
import { validateDocFile } from "@/lib/validation/schemas";
import { formatDate } from "@/lib/utils/format";
import type { Lang } from "@/types";

export function ChildDetailPage() {
  const { id } = useParams<{ id: string }>();
  const t = useTranslations("children");
  const toast = useToast();
  const qc = useQueryClient();
  const locale = useLocale() as Lang;
  const key = ["agency", "child", id];
  const q = useQuery({ queryKey: key, queryFn: ({ signal }) => agencyApi.child(id, signal) });
  const refresh = () => { qc.invalidateQueries({ queryKey: key }); qc.invalidateQueries({ queryKey: ["agency", "children"] }); };

  const [csr, setCsr] = React.useState<File | null>(null);
  const [mer, setMer] = React.useState<File | null>(null);
  const [progress, setProgress] = React.useState(0);
  const [summary, setSummary] = React.useState("");
  const [aiRunId, setAiRunId] = React.useState<string | null>(null);
  const [referralOpen, setReferralOpen] = React.useState(false);

  const fileError = (f: File | null) => { const e = f ? validateDocFile(f) : null; return e ? t(`fileError.${e}`) : null; };
  const csrErr = fileError(csr), merErr = fileError(mer);

  const upload = useMutation({
    mutationFn: () => agencyApi.uploadReports(id, csr!, mer!, { onProgress: setProgress }),
    onSuccess: (r) => { toast({ tone: "success", title: t("reportsUploaded"), body: r.mer_text_extracted ? t("merExtracted") : t("merNotExtracted") }); setCsr(null); setMer(null); setProgress(0); refresh(); },
    onError: (e) => { setProgress(0); toast({ tone: "error", title: t("uploadFailed"), body: e.message }); },
  });
  const draft = useMutation({
    mutationFn: () => aiApi.medicalSummary({ child_id: id, language: locale }),
    onSuccess: (r) => { setSummary(r.draft); setAiRunId(r.ai_run_id); toast({ tone: "ai", title: t("draftReady") }); },
    onError: (e) => toast({ tone: "error", title: t("draftFailed"), body: e instanceof ApiError && e.code === "AI_UNAVAILABLE" ? t("aiDown") : e.message }),
  });
  const approve = useMutation({
    mutationFn: () => agencyApi.approveSummary(id, { approved_text: summary, ai_run_id: aiRunId! }),
    onSuccess: (r) => { toast({ tone: "success", title: t("approved"), body: t("childStatus", { status: r.child_status }) }); refresh(); },
    onError: (e) => toast({ tone: "error", title: t("approveFailed"), body: e.message }),
  });

  if (q.isPending) return <PageSkeleton />;
  if (q.isError) return <ErrorState error={q.error} onRetry={() => q.refetch()} />;
  const c = q.data;
  const hasSummary = !!c.summary_approved;
  const locked = ["READY", "REFERRED", "RESERVED", "PLACED"].includes(c.status);
  const checklist = [
    { ok: c.has_csr, label: t("lockCsr") },
    { ok: c.has_mer, label: t("lockMer") },
    { ok: hasSummary, label: t("lockSummary") },
  ];

  return (
    <>
      <Link href="/agency/children" className="mb-4 inline-flex items-center gap-1 text-sm font-semibold text-purple-deep"><ArrowLeft className="h-4 w-4" aria-hidden /> {t("back")}</Link>
      <PageHead title={c.case_code} subtitle={`${c.age_text} | ${t(`gender.${c.gender}`)}`}>
        <Badge tone={c.status === "READY" ? "success" : "neutral"}>{t(`status.${c.status}`)}</Badge>
      </PageHead>

      <div className="grid gap-6 lg:grid-cols-[1.3fr_1fr]">
        <div className="space-y-6">
          <Card>
            <h2 className="flex items-center gap-2 text-lg font-bold"><FileUp className="h-5 w-5" aria-hidden /> {t("reportsTitle")}</h2>
            <p className="mt-1 text-xs text-muted">{t("reportsNote")}</p>
            <div className="mt-4 grid gap-4 sm:grid-cols-2">
              <Field label={t("csrFile")} htmlFor="csr" error={csrErr ?? undefined}><input id="csr" type="file" accept="application/pdf,image/jpeg,image/png" onChange={(e) => setCsr(e.target.files?.[0] ?? null)} className="block w-full text-sm" /></Field>
              <Field label={t("merFile")} htmlFor="mer" error={merErr ?? undefined}><input id="mer" type="file" accept="application/pdf,image/jpeg,image/png" onChange={(e) => setMer(e.target.files?.[0] ?? null)} className="block w-full text-sm" /></Field>
            </div>
            {upload.isPending && <div className="mt-3"><ProgressBar value={progress} label={t("uploading")} /></div>}
            <Button className="mt-4" onClick={() => upload.mutate()} disabled={!csr || !mer || !!csrErr || !!merErr} loading={upload.isPending}>{t("uploadReports")}</Button>
          </Card>

          <Card>
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h2 className="flex items-center gap-2 text-lg font-bold"><Sparkles className="h-5 w-5" aria-hidden /> {t("summaryTitle")}</h2>
              <AiLabel />
            </div>
            {hasSummary ? (
              <>
                <InlineAlert tone="success" className="mt-3" title={t("approvedSummary")} />
                <p className="mt-3 whitespace-pre-line text-sm">{c.summary_approved}</p>
              </>
            ) : (
              <div className="mt-3 space-y-3">
                <p className="text-xs text-muted">{t("summaryNote")}</p>
                <Button variant="purple" onClick={() => draft.mutate()} loading={draft.isPending} disabled={!c.has_mer}><Sparkles className="h-4 w-4" aria-hidden /> {t("draftSummary")}</Button>
                {!c.has_mer && <p className="text-xs text-muted">{t("needMer")}</p>}
                {draft.isPending && <ProgressBar indeterminate label={t("drafting")} />}
                {(summary || aiRunId) && (
                  <>
                    <Field label={t("editSummary")} htmlFor="summary" hint={t("editSummaryHint")}>
                      <Textarea id="summary" rows={10} value={summary} onChange={(e) => setSummary(e.target.value)} />
                    </Field>
                    <Button onClick={() => approve.mutate()} loading={approve.isPending} disabled={!summary.trim() || !aiRunId}><Check className="h-4 w-4" aria-hidden /> {t("approve")}</Button>
                  </>
                )}
              </div>
            )}
          </Card>
        </div>

        <div className="space-y-6">
          <Card variant={locked ? "mint" : "sun"}>
            <h2 className="text-lg font-bold">{t("lockTitle")}</h2>
            <p className="mt-1 text-xs text-ink-soft">{t("lockNote")}</p>
            <ul className="mt-3 space-y-2">
              {checklist.map((x) => (
                <li key={x.label} className="flex items-center gap-2 rounded-2xl bg-white p-3 text-sm font-semibold">
                  {x.ok ? <Check className="h-4 w-4 text-success" aria-hidden /> : <Circle className="h-4 w-4 text-faint" aria-hidden />}
                  {x.label}
                  <span className="sr-only">{x.ok ? t("done") : t("pending")}</span>
                </li>
              ))}
            </ul>
          </Card>
          <Card>
            <h2 className="flex items-center gap-2 text-lg font-bold"><UserSearch className="h-5 w-5" aria-hidden /> {t("referralTitle")}</h2>
            <p className="mt-1 text-xs text-muted">{t("referralNote")}</p>
            <Button className="mt-4 w-full" onClick={() => setReferralOpen(true)} disabled={c.status !== "READY"}><Send className="h-4 w-4" aria-hidden /> {t("findParent")}</Button>
            {c.status !== "READY" && <p className="mt-2 text-xs text-muted">{t("notReady")}</p>}
          </Card>
        </div>
      </div>
      {referralOpen && <ReferralSheet childId={id} onClose={() => setReferralOpen(false)} onSent={refresh} />}
    </>
  );
}

function ReferralSheet({ childId, onClose, onSent }: { childId: string; onClose: () => void; onSent: () => void }) {
  const t = useTranslations("children");
  const locale = useLocale();
  const toast = useToast();
  const next = useQuery({ queryKey: ["agency", "next-parent", childId], queryFn: () => agencyApi.nextParent(childId), retry: false });
  const send = useMutation({
    mutationFn: () => agencyApi.sendReferral({ child_id: childId, application_id: next.data!.application_id }),
    onSuccess: (r) => { toast({ tone: "success", title: t("referralSent"), body: t("expiresAt", { date: formatDate(r.expires_at, locale) }) }); onSent(); onClose(); },
    onError: (e) => toast({ tone: "error", title: e instanceof ApiError && e.status === 409 ? t("referralLocked") : t("referralFailed"), body: e.message }),
  });
  return (
    <Sheet
      open
      onClose={() => !send.isPending && onClose()}
      title={t("nextParentTitle")}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={send.isPending}>{t("cancel")}</Button>
          <Button onClick={() => send.mutate()} loading={send.isPending} disabled={!next.data}>{t("sendReferral")}</Button>
        </>
      }
    >
      {next.isPending && <p>{t("loading")}</p>}
      {next.isError && <ErrorState compact error={next.error} onRetry={() => next.refetch()} />}
      {next.data && (
        <div className="space-y-3">
          <InlineAlert tone="purple">{t("ruleBased")}</InlineAlert>
          <dl className="grid grid-cols-2 gap-3 text-sm">
            <div className="rounded-2xl bg-canvas p-3"><dt className="text-xs font-bold uppercase text-muted">{t("parent")}</dt><dd className="font-semibold">{next.data.parent_name}</dd></div>
            <div className="rounded-2xl bg-canvas p-3"><dt className="text-xs font-bold uppercase text-muted">{t("seniority")}</dt><dd className="font-semibold">{formatDate(next.data.seniority_date, locale)}</dd></div>
          </dl>
          <p className="text-xs text-muted">{t("matchedOn", { fields: next.data.matched_on.join(", ") })}</p>
          <p className="text-xs font-semibold">{t("timerStarts")}</p>
        </div>
      )}
    </Sheet>
  );
}
