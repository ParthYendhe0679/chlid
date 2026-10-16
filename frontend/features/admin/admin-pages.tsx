"use client";

import * as React from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { AlertOctagon, CheckCircle2, RefreshCw, ShieldCheck, TrendingUp } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card, PageHead } from "@/components/ui/card";
import { Field, Input, Select, Textarea } from "@/components/ui/form";
import { Badge, EmptyState, ErrorState, InlineAlert, Skeleton, StatCard } from "@/components/ui/feedback";
import { useToast } from "@/components/ui/toast";
import { adminApi } from "@/lib/api/admin.api";
import { knowledgeSchema } from "@/lib/validation/schemas";
import { useFieldError } from "@/lib/utils/use-form-error";
import { formatDate, formatDateTime } from "@/lib/utils/format";
import { RiskReasons, HIGH_RISK_THRESHOLD } from "@/features/deadlines/risk-reasons";
import { DeadlineBadge } from "@/features/case-tracker/deadline-badge";
import { z } from "zod";

const STEPS = ["DOC_UPLOAD_30D", "HSR_60D", "CSR_MER_10D", "RESERVE_48H", "MATCHING_30D", "FOSTER_CARE_10D", "DM_FILING_10D", "DM_ORDER_60D", "FOLLOWUP_VISIT"];

export function RadarPage() {
  const t = useTranslations("admin");
  const td = useTranslations("deadline");
  const toast = useToast();
  const qc = useQueryClient();
  const [step, setStep] = React.useState("");
  const [level, setLevel] = React.useState("");
  const [search, setSearch] = React.useState("");
  const radar = useQuery({ queryKey: ["admin", "radar", step, level], queryFn: ({ signal }) => adminApi.radar({ step: step || undefined, level: level || undefined }, signal) });
  const score = useQuery({ queryKey: ["admin", "scorecard"], queryFn: ({ signal }) => adminApi.scorecard(signal) });
  const recompute = useMutation({
    mutationFn: adminApi.recompute,
    onSuccess: (r) => { toast({ tone: "success", title: t("recomputed", { count: r.updated }) }); qc.invalidateQueries({ queryKey: ["admin"] }); },
    onError: (e) => toast({ tone: "error", title: t("recomputeFailed"), body: e.message }),
  });

  const items = (radar.data?.items ?? []).filter((i) => !search || i.case_ref.toLowerCase().includes(search.toLowerCase()) || i.agency.toLowerCase().includes(search.toLowerCase()));
  const chart = (score.data ?? []).map((a) => ({ name: a.agency, "HSR": a.avg_days["HSR_60D"] ?? 0, "DM": a.avg_days["DM_ORDER_60D"] ?? 0 }));

  return (
    <>
      <PageHead kicker={t("kicker")} title={t("radarTitle")} subtitle={t("radarSubtitle")}>
        <Button variant="secondary" onClick={() => recompute.mutate()} loading={recompute.isPending}><RefreshCw className="h-4 w-4" aria-hidden /> {t("recompute")}</Button>
      </PageHead>
      <InlineAlert tone="neutral" className="mb-5">{t("distinction")}</InlineAlert>

      {radar.isPending && <div className="grid gap-4 sm:grid-cols-3"><Skeleton className="h-28" /><Skeleton className="h-28" /><Skeleton className="h-28" /></div>}
      {radar.isError && <ErrorState error={radar.error} onRetry={() => radar.refetch()} />}
      {radar.isSuccess && (
        <>
          <div className="grid gap-4 sm:grid-cols-3">
            <StatCard label={t("overdue")} icon={AlertOctagon} tone="coral" value={radar.data.summary.overdue} hint={t("overdueHint")} />
            <StatCard label={t("highRisk")} icon={TrendingUp} tone="sun" value={radar.data.summary.high_risk} hint={t("highRiskHint", { p: HIGH_RISK_THRESHOLD })} />
            <StatCard label={t("onTrack")} icon={CheckCircle2} tone="mint" value={radar.data.summary.on_track} />
          </div>

          <Card variant="bento" pad={false} className="mt-6 overflow-hidden">
            <div className="flex flex-wrap items-end gap-3 p-4">
              <Field label={t("search")} htmlFor="r-search" className="w-56"><Input id="r-search" value={search} onChange={(e) => setSearch(e.target.value)} className="h-10 rounded-full" /></Field>
              <Field label={t("step")} htmlFor="r-step" className="w-56"><Select id="r-step" value={step} onChange={(e) => setStep(e.target.value)} className="rounded-full"><option value="">{t("all")}</option>{STEPS.map((s) => <option key={s} value={s}>{td(`step.${s}`)}</option>)}</Select></Field>
              <Field label={t("level")} htmlFor="r-level" className="w-48"><Select id="r-level" value={level} onChange={(e) => setLevel(e.target.value)} className="rounded-full"><option value="">{t("all")}</option><option value="OVERDUE">{t("overdue")}</option><option value="HIGH_RISK">{t("highRisk")}</option><option value="ON_TRACK">{t("onTrack")}</option></Select></Field>
            </div>
            {items.length === 0 ? <div className="p-4"><EmptyState title={t("noItems")} /></div> : (
              <div className="overflow-x-auto">
                <table className="w-full min-w-[860px] text-sm">
                  <thead className="bg-canvas text-left text-[11px] font-bold uppercase tracking-wider text-muted">
                    <tr><th className="px-4 py-3">{t("case")}</th><th className="px-4 py-3">{t("agency")}</th><th className="px-4 py-3">{t("step")}</th><th className="px-4 py-3">{t("deadline")}</th><th className="px-4 py-3">{t("prediction")}</th></tr>
                  </thead>
                  <tbody>
                    {items.map((i) => (
                      <tr key={i.case_ref + i.step} className="border-t border-line align-top transition-colors hover:bg-canvas/60">
                        <td className="px-4 py-3 font-mono font-bold">{i.case_ref}</td>
                        <td className="px-4 py-3">{i.agency}</td>
                        <td className="px-4 py-3">{td.has(`step.${i.step}`) ? td(`step.${i.step}`) : i.step}</td>
                        <td className="px-4 py-3"><DeadlineBadge showStep={false} deadline={i} /></td>
                        <td className="px-4 py-3"><RiskReasons score={i.risk_score} reasons={i.risk_reasons} /></td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Card>
        </>
      )}

      <Card variant="bento" className="mt-6">
        <h2 className="text-lg font-bold">{t("scorecard")}</h2>
        <p className="text-xs text-muted">{t("scorecardNote")}</p>
        {score.isPending && <Skeleton className="mt-3 h-56" />}
        {score.isError && <ErrorState compact error={score.error} onRetry={() => score.refetch()} />}
        {score.isSuccess && chart.length === 0 && <EmptyState title={t("noItems")} />}
        {score.isSuccess && chart.length > 0 && (
          <div className="mt-3" role="img" aria-label={t("scorecard")}>
            <ResponsiveContainer width="100%" height={220}>
              <BarChart data={chart}>
                <CartesianGrid stroke="#eef1f6" vertical={false} />
                <XAxis dataKey="name" tick={{ fontSize: 11 }} />
                <YAxis tick={{ fontSize: 11 }} />
                <Tooltip cursor={{ fill: "#f5f1e8" }} contentStyle={{ borderRadius: 14, border: "1px solid #e3e7ef", boxShadow: "0 8px 24px -12px rgba(28,27,43,.25)", fontSize: 12 }} />
                <Bar dataKey="HSR" fill="#7b5cf0" radius={[8, 8, 0, 0]} maxBarSize={36} />
                <Bar dataKey="DM" fill="#f08a6c" radius={[8, 8, 0, 0]} maxBarSize={36} />
              </BarChart>
            </ResponsiveContainer>
            <p className="mt-1 text-[11px] text-muted">{t("legalLimit")}</p>
          </div>
        )}
      </Card>
    </>
  );
}

export function ChildAlertsPage() {
  const t = useTranslations("admin");
  const q = useQuery({ queryKey: ["admin", "alerts"], queryFn: ({ signal }) => adminApi.childAlerts(signal) });
  return (
    <>
      <PageHead kicker={t("kicker")} title={t("alertsTitle")} subtitle={t("alertsSubtitle")} />
      {q.isPending && <Skeleton className="h-48" />}
      {q.isError && <ErrorState error={q.error} onRetry={() => q.refetch()} />}
      {q.isSuccess && q.data.length === 0 && <EmptyState title={t("noAlerts")} />}
      {q.isSuccess && q.data.length > 0 && (
        <ul className="grid gap-3 sm:grid-cols-2">
          {q.data.map((a) => (
            <li key={a.case_code + a.reason}>
              <Card variant="bento">
                <p className="font-mono font-bold">{a.case_code}</p>
                <p className="text-xs text-muted">{a.agency}</p>
                <div className="mt-2 flex items-center gap-2"><Badge tone="warning">{t(`reason.${a.reason}`)}</Badge><span className="tabular text-xs">{t("days", { days: a.days })}</span></div>
              </Card>
            </li>
          ))}
        </ul>
      )}
      <p className="mt-4 text-xs text-muted">{t("maskedNote")}</p>
    </>
  );
}

export function AgenciesPage() {
  const t = useTranslations("admin");
  const toast = useToast();
  const qc = useQueryClient();
  const q = useQuery({ queryKey: ["admin", "agencies"], queryFn: ({ signal }) => adminApi.agencies(false, signal) });
  const m = useMutation({
    mutationFn: ({ id, ok }: { id: string; ok: boolean }) => adminApi.setApproval(id, ok),
    onSuccess: () => { toast({ tone: "success", title: t("updated") }); qc.invalidateQueries({ queryKey: ["admin", "agencies"] }); },
    onError: (e) => toast({ tone: "error", title: t("updateFailed"), body: e.message }),
  });
  return (
    <>
      <PageHead kicker={t("kicker")} title={t("agenciesTitle")} subtitle={t("agenciesSubtitle")} />
      {q.isPending && <Skeleton className="h-48" />}
      {q.isError && <ErrorState error={q.error} onRetry={() => q.refetch()} />}
      {q.isSuccess && q.data.length === 0 && <EmptyState title={t("noPending")} />}
      {q.isSuccess && q.data.length > 0 && (
        <ul className="space-y-3">
          {q.data.map((a) => (
            <li key={a.id}>
              <Card variant="bento" className="flex flex-wrap items-center justify-between gap-3">
                <div><p className="font-bold">{a.name}</p><p className="text-xs text-muted"><span className="font-mono">{a.reg_number}</span> | {a.agency_type} | {a.district}, {a.state}</p></div>
                <div className="flex gap-2">
                  <Button size="sm" variant="outline" disabled={m.isPending} onClick={() => m.mutate({ id: a.id, ok: false })}>{t("reject")}</Button>
                  <Button size="sm" variant="success" disabled={m.isPending} onClick={() => m.mutate({ id: a.id, ok: true })}>{t("approve")}</Button>
                </div>
              </Card>
            </li>
          ))}
        </ul>
      )}
    </>
  );
}

export function KnowledgePage() {
  const t = useTranslations("admin");
  const fe = useFieldError();
  const toast = useToast();
  const { register, handleSubmit, reset, formState: { errors } } = useForm<z.infer<typeof knowledgeSchema>>({ resolver: zodResolver(knowledgeSchema), defaultValues: { language: "en" } });
  const m = useMutation({
    mutationFn: (v: z.infer<typeof knowledgeSchema>) => adminApi.addKnowledge({ ...v, source_url: v.source_url || undefined }),
    onSuccess: (r) => { toast({ tone: "success", title: t("kbDone", { count: r.chunks_created }) }); reset(); },
    onError: (e) => toast({ tone: "error", title: t("kbFailed"), body: e.message }),
  });
  return (
    <>
      <PageHead kicker={t("kicker")} title={t("kbTitle")} subtitle={t("kbSubtitle")} />
      <Card className="max-w-3xl">
        <form onSubmit={handleSubmit((v) => m.mutate(v))} noValidate className="space-y-4">
          <Field label={t("kbSource")} htmlFor="kb-title" error={fe(errors.source_title)}><Input id="kb-title" {...register("source_title")} /></Field>
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label={t("kbUrl")} htmlFor="kb-url" error={fe(errors.source_url)}><Input id="kb-url" {...register("source_url")} /></Field>
            <Field label={t("kbLang")} htmlFor="kb-lang"><Select id="kb-lang" {...register("language")}><option value="en">English</option><option value="hi">हिन्दी</option><option value="mr">मराठी</option></Select></Field>
          </div>
          <Field label={t("kbText")} hint={t("kbHint")} htmlFor="kb-text" error={fe(errors.text)}><Textarea id="kb-text" rows={12} {...register("text")} /></Field>
          <Button type="submit" loading={m.isPending}>{t("kbSubmit")}</Button>
          {m.isPending && <p className="text-xs text-muted" role="status">{t("kbProcessing")}</p>}
          <InlineAlert tone="info">{t("kbActive")}</InlineAlert>
        </form>
      </Card>
    </>
  );
}

export function ScamReportsPage() {
  const t = useTranslations("admin");
  const locale = useLocale();
  const q = useQuery({ queryKey: ["admin", "scams"], queryFn: ({ signal }) => adminApi.scamReports(signal) });
  return (
    <>
      <PageHead kicker={t("kicker")} title={t("scamsTitle")} subtitle={t("scamsSubtitle")} />
      {q.isPending && <Skeleton className="h-48" />}
      {q.isError && <ErrorState error={q.error} onRetry={() => q.refetch()} />}
      {q.isSuccess && q.data.length === 0 && <EmptyState title={t("noScams")} />}
      {q.isSuccess && q.data.length > 0 && (
        <ul className="space-y-3">
          {q.data.map((r) => (
            <li key={r.id}>
              <Card variant="bento">
                <div className="flex flex-wrap items-center justify-between gap-2"><Badge tone={r.verdict === "LIKELY_SCAM" ? "danger" : "warning"}>{r.verdict}</Badge><span className="text-xs text-muted">{formatDateTime(r.created_at, locale)}</span></div>
                <p className="mt-3 whitespace-pre-wrap break-words rounded-2xl bg-canvas p-3 text-sm">{r.text}</p>
                {r.reasons.length > 0 && <ul className="mt-2 list-disc pl-5 text-xs text-ink-soft">{r.reasons.map((x) => <li key={x}>{x}</li>)}</ul>}
                <p className="mt-2 text-[11px] text-muted">{t("aiSuspicion")}</p>
              </Card>
            </li>
          ))}
        </ul>
      )}
    </>
  );
}

export function AuditPage() {
  const t = useTranslations("admin");
  const locale = useLocale();
  const [f, setF] = React.useState({ actor: "", action: "", from: "", to: "" });
  const [applied, setApplied] = React.useState(f);
  const q = useQuery({ queryKey: ["admin", "audit", applied], queryFn: ({ signal }) => adminApi.audit({ actor: applied.actor || undefined, action: applied.action || undefined, from: applied.from || undefined, to: applied.to || undefined }, signal) });
  const verify = useMutation({ mutationFn: adminApi.verifyAudit });
  return (
    <>
      <PageHead kicker={t("kicker")} title={t("auditTitle")} subtitle={t("auditSubtitle")}>
        <Button onClick={() => verify.mutate()} loading={verify.isPending}><ShieldCheck className="h-4 w-4" aria-hidden /> {t("verifyChain")}</Button>
      </PageHead>
      {verify.isError && <ErrorState compact error={verify.error} />}
      {verify.data && (verify.data.valid
        ? <InlineAlert tone="success" className="mb-5" title={t("chainValid")}>{t("rowsChecked", { count: verify.data.rows_checked ?? 0 })}</InlineAlert>
        : <InlineAlert tone="danger" className="mb-5" title={t("chainBroken")}>{t("brokenAt", { id: verify.data.broken_at_id ?? 0 })}</InlineAlert>)}
      <Card variant="bento" pad={false} className="overflow-hidden">
        <form className="flex flex-wrap items-end gap-3 p-4" onSubmit={(e) => { e.preventDefault(); setApplied(f); }}>
          <Field label={t("actor")} htmlFor="a-actor" className="w-48"><Input id="a-actor" value={f.actor} onChange={(e) => setF({ ...f, actor: e.target.value })} className="h-10" /></Field>
          <Field label={t("action")} htmlFor="a-action" className="w-48"><Input id="a-action" value={f.action} onChange={(e) => setF({ ...f, action: e.target.value })} className="h-10" /></Field>
          <Field label={t("from")} htmlFor="a-from"><Input id="a-from" type="date" value={f.from} onChange={(e) => setF({ ...f, from: e.target.value })} className="h-10" /></Field>
          <Field label={t("to")} htmlFor="a-to"><Input id="a-to" type="date" value={f.to} onChange={(e) => setF({ ...f, to: e.target.value })} className="h-10" /></Field>
          <Button type="submit" size="sm">{t("filter")}</Button>
        </form>
        {q.isPending && <div className="p-4"><Skeleton className="h-40" /></div>}
        {q.isError && <div className="p-4"><ErrorState error={q.error} onRetry={() => q.refetch()} /></div>}
        {q.isSuccess && q.data.length === 0 && <div className="p-4"><EmptyState title={t("noAudit")} /></div>}
        {q.isSuccess && q.data.length > 0 && (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[760px] text-sm">
              <thead className="bg-canvas text-left text-[11px] font-bold uppercase tracking-wider text-muted"><tr><th className="px-4 py-3">#</th><th className="px-4 py-3">{t("when")}</th><th className="px-4 py-3">{t("actor")}</th><th className="px-4 py-3">{t("action")}</th><th className="px-4 py-3">{t("target")}</th></tr></thead>
              <tbody>
                {q.data.map((e) => (
                  <tr key={e.id} className="border-t border-line">
                    <td className="tabular px-4 py-2 font-mono">{e.id}</td>
                    <td className="tabular px-4 py-2">{formatDateTime(e.created_at, locale)}</td>
                    <td className="px-4 py-2">{e.actor_role ?? "-"}<span className="ml-1 font-mono text-[11px] text-muted">{e.actor_id?.slice(0, 8)}</span></td>
                    <td className="px-4 py-2 font-mono font-bold">{e.action}</td>
                    <td className="px-4 py-2 font-mono text-xs">{e.target_type}:{e.target_id}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
      <p className="mt-3 text-xs text-muted">{t("auditReadOnly")}</p>
    </>
  );
}
