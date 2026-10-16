"use client";

import * as React from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Check, Eye, RotateCcw, ShieldCheck } from "lucide-react";
import { useFormatter, useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Field, Textarea } from "@/components/ui/form";
import { AiLabel, InlineAlert } from "@/components/ui/feedback";
import { Sheet } from "@/components/ui/sheet";
import { useToast } from "@/components/ui/toast";
import { agencyApi } from "@/lib/api/agency.api";
import { applicationsApi } from "@/lib/api/applications.api";
import { reuploadNoteSchema } from "@/lib/validation/schemas";
import { useFieldError } from "@/lib/utils/use-form-error";
import { CheckList, DocStatusBadge } from "@/features/documents/check-list";
import type { AgencyCaseDetail, CaseDocument } from "@/types";
import { z } from "zod";

export function DocumentReview({ detail }: { detail: AgencyCaseDetail }) {
  const t = useTranslations("review");
  const tc = useTranslations("documents");
  const toast = useToast();
  const qc = useQueryClient();
  const [reupload, setReupload] = React.useState<CaseDocument | null>(null);
  const caseId = detail.application_id;
  const refresh = () => { qc.invalidateQueries({ queryKey: ["agency", "case", caseId] }); qc.invalidateQueries({ queryKey: ["agency", "cases"] }); };

  const verify = useMutation({
    mutationFn: (id: string) => agencyApi.reviewDocument(id, { decision: "VERIFIED" }),
    onSuccess: () => { toast({ tone: "success", title: t("verified") }); refresh(); },
    onError: (e) => toast({ tone: "error", title: t("reviewFailed"), body: e.message }),
  });
  const verifyAll = useMutation({
    mutationFn: () => agencyApi.verifyAll(caseId),
    onSuccess: (r) => { toast({ tone: "success", title: t("allVerified"), body: t("hsrStarted", { status: r.status }) }); refresh(); },
    onError: (e) => toast({ tone: "error", title: t("verifyAllFailed"), body: e.message }),
  });

  const docs = detail.documents;
  const allVerified = docs.length > 0 && docs.every((d) => d.status === "VERIFIED");

  return (
    <div className="space-y-5">
      <InlineAlert tone="purple">{t("humanDecides")}</InlineAlert>
      {docs.length === 0 && <InlineAlert tone="info">{t("noDocs")}</InlineAlert>}
      {docs.map((d) => (
        <DocRow key={d.id} doc={d} busy={verify.isPending} onVerify={() => verify.mutate(d.id)} onReupload={() => setReupload(d)} label={tc(`categories.${d.category}.title`)} />
      ))}
      {docs.length > 0 && detail.status === "DOCS_SUBMITTED" && (
        <div className="flex justify-end">
          <Button onClick={() => verifyAll.mutate()} disabled={!allVerified} loading={verifyAll.isPending}><ShieldCheck className="h-4 w-4" aria-hidden /> {t("verifyAll")}</Button>
        </div>
      )}
      <ReuploadSheet doc={reupload} onClose={() => setReupload(null)} onDone={() => { setReupload(null); refresh(); }} />
    </div>
  );
}

function DocRow({ doc, label, busy, onVerify, onReupload }: { doc: CaseDocument; label: string; busy: boolean; onVerify: () => void; onReupload: () => void }) {
  const t = useTranslations("review");
  const fmt = useFormatter();
  const toast = useToast();
  const [preview, setPreview] = React.useState<{ url: string; kind: "image" | "pdf" } | null>(null);
  const [loading, setLoading] = React.useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const { url } = await applicationsApi.documentUrl(doc.id);
      setPreview({ url, kind: /\.pdf$/i.test(doc.file_name) ? "pdf" : "image" });
    } catch (e) {
      toast({ tone: "error", title: t("previewFailed"), body: (e as Error).message });
    } finally { setLoading(false); }
  };

  const lowConf = doc.ai_confidence != null && doc.ai_confidence < 0.85;
  const fields = Object.entries(doc.ai_fields ?? {}).filter(([, v]) => v !== null && v !== "");

  return (
    <Card variant="bento" className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div><h3 className="font-bold">{label}</h3><p className="font-mono text-[11px] text-muted">{doc.file_name}</p></div>
        <DocStatusBadge status={doc.status} />
      </div>
      <div className="grid gap-4 lg:grid-cols-2">
        <div className="min-h-48 rounded-2xl bg-canvas p-3">
          <p className="mb-2 text-xs font-bold uppercase tracking-wider text-muted">{t("document")}</p>
          {!preview ? (
            <Button variant="secondary" size="sm" onClick={load} loading={loading}><Eye className="h-4 w-4" aria-hidden /> {t("showDocument")}</Button>
          ) : preview.kind === "image" ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={preview.url} alt={label} className="max-h-96 w-full rounded-xl object-contain" referrerPolicy="no-referrer" />
          ) : (
            <iframe src={preview.url} title={label} sandbox="" referrerPolicy="no-referrer" className="h-96 w-full rounded-xl bg-white" />
          )}
          <p className="mt-2 text-[11px] text-muted">{t("linkExpires")}</p>
        </div>
        <div className="space-y-3">
          <div className="flex flex-wrap items-center gap-2"><AiLabel /><p className="text-xs text-muted">{t("aiFindings")}</p></div>
          {doc.ai_detected_type && (
            <p className="text-sm">{t("detected")}: <strong className="font-mono">{doc.ai_detected_type}</strong>{doc.ai_confidence != null && <> ({fmt.number(doc.ai_confidence, { style: "percent" })})</>}</p>
          )}
          {lowConf && <InlineAlert tone="warning">{t("lowConfidence")}</InlineAlert>}
          {fields.length > 0 && (
            <dl className="grid grid-cols-2 gap-2 text-xs">
              {fields.map(([k, v]) => (
                <div key={k} className="rounded-xl bg-white p-2 shadow-clay-sm"><dt className="font-bold uppercase text-muted">{k}</dt><dd className="mt-0.5 break-words font-mono">{String(v)}</dd></div>
              ))}
            </dl>
          )}
          <CheckList checks={doc.ai_checks} />
          {doc.reviewer_note && <p className="text-xs text-muted">{t("note")}: {doc.reviewer_note}</p>}
        </div>
      </div>
      <div className="flex flex-wrap justify-end gap-2 border-t border-line pt-3">
        <Button variant="outline" size="sm" onClick={onReupload} disabled={busy || doc.status === "REUPLOAD_REQUESTED"}><RotateCcw className="h-4 w-4" aria-hidden /> {t("askReupload")}</Button>
        <Button variant="success" size="sm" onClick={onVerify} disabled={busy || doc.status === "VERIFIED"}><Check className="h-4 w-4" aria-hidden /> {t("verify")}</Button>
      </div>
    </Card>
  );
}

function ReuploadSheet({ doc, onClose, onDone }: { doc: CaseDocument | null; onClose: () => void; onDone: () => void }) {
  const t = useTranslations("review");
  const fe = useFieldError();
  const toast = useToast();
  const form = useForm<z.infer<typeof reuploadNoteSchema>>({ resolver: zodResolver(reuploadNoteSchema) });
  const m = useMutation({
    mutationFn: (note: string) => agencyApi.reviewDocument(doc!.id, { decision: "REUPLOAD_REQUESTED", note }),
    onSuccess: () => { toast({ tone: "success", title: t("reuploadSent") }); form.reset(); onDone(); },
    onError: (e) => toast({ tone: "error", title: t("reviewFailed"), body: e.message }),
  });
  return (
    <Sheet
      open={!!doc}
      onClose={() => !m.isPending && onClose()}
      title={t("askReupload")}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={m.isPending}>{t("cancel")}</Button>
          <Button loading={m.isPending} onClick={form.handleSubmit((v) => m.mutate(v.note))}>{t("sendRequest")}</Button>
        </>
      }
    >
      <Field label={t("reasonLabel")} hint={t("reasonHint")} htmlFor="note" error={fe(form.formState.errors.note)}>
        <Textarea id="note" rows={4} aria-invalid={!!form.formState.errors.note} {...form.register("note")} />
      </Field>
    </Sheet>
  );
}
