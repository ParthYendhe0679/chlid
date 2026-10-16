"use client";

import * as React from "react";
import { Eye, FileUp, RotateCcw, X } from "lucide-react";
import { useFormatter, useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { AiLabel, InlineAlert } from "@/components/ui/feedback";
import { useToast } from "@/components/ui/toast";
import { applicationsApi } from "@/lib/api/applications.api";
import { ALLOWED_DOC_TYPES, MAX_DOC_BYTES } from "@/lib/validation/schemas";
import { cn } from "@/lib/utils/cn";
import { CheckList, DocStatusBadge } from "./check-list";
import { TerminalPanel } from "./terminal-panel";
import { useDocCheck, type RunText } from "./use-doccheck";
import type { DocCategory, RequiredDoc } from "@/types";

export function UploadCard({ doc, canUpload }: { doc: RequiredDoc; canUpload: boolean }) {
  const t = useTranslations("doccheck");
  const tc = useTranslations("documents");
  const toast = useToast();
  const fmt = useFormatter();
  const inputRef = React.useRef<HTMLInputElement>(null);
  const [drag, setDrag] = React.useState(false);
  const [opening, setOpening] = React.useState(false);
  const category = doc.category as DocCategory;

  const text: RunText = {
    localOk: ({ type, size }) => t("log.localOk", { type, size }),
    localBad: (r) => t(`log.localBad.${r}`),
    uploadStart: t("log.uploadStart"),
    uploadPct: (pct) => t("log.uploadPct", { pct }),
    uploadSent: t("log.uploadSent"),
    serverWaiting: t("log.serverWaiting"),
    check: (c) => `${c.code}: ${c.message}`,
    resultLine: ({ status, type, confidence }) =>
      t("log.result", { status: t(`status.${status}`), type: type ?? "-", confidence: confidence != null ? fmt.number(confidence, { style: "percent" }) : "-" }),
    failed: (m) => t("log.failed", { message: m }),
    cancelled: t("log.cancelled"),
  };
  const { state, run, cancel, reset, busy } = useDocCheck(category, text);

  const pick = (files: FileList | null) => {
    const f = files?.[0];
    if (f) void run(f);
    if (inputRef.current) inputRef.current.value = "";
  };

  const view = async () => {
    if (!doc.document_id) return;
    setOpening(true);
    try {
      const { url } = await applicationsApi.documentUrl(doc.document_id);
      window.open(url, "_blank", "noopener,noreferrer"); // short-lived link, never stored
    } catch (e) {
      toast({ tone: "error", title: t("viewFailed"), body: (e as Error).message });
    } finally {
      setOpening(false);
    }
  };

  const showTerminal = state.phase !== "idle";
  const result = state.result;
  const lockedByAgency = doc.status === "VERIFIED";

  return (
    <Card variant="bento" className="space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h3 className="text-base font-bold">{tc(`categories.${category}.title`)}</h3>
          <p className="mt-0.5 max-w-xl text-xs text-muted">{tc(`categories.${category}.hint`)}</p>
        </div>
        {doc.uploaded && doc.status ? <DocStatusBadge status={doc.status} /> : <span className="text-xs font-bold text-muted">{tc("notUploaded")}</span>}
      </div>

      {doc.status === "REUPLOAD_REQUESTED" && doc.reviewer_note && (
        <InlineAlert tone="warning" title={tc("reuploadRequested")}>{doc.reviewer_note}</InlineAlert>
      )}
      {lockedByAgency && <InlineAlert tone="success">{tc("verifiedLocked")}</InlineAlert>}

      {canUpload && !lockedByAgency && !busy && (
        <div
          onDragOver={(e) => { e.preventDefault(); setDrag(true); }}
          onDragLeave={() => setDrag(false)}
          onDrop={(e) => { e.preventDefault(); setDrag(false); pick(e.dataTransfer.files); }}
          className={cn("rounded-2xl border-2 border-dashed p-4 text-center transition-colors", drag ? "border-purple bg-lavender-soft" : "border-ink/20 bg-canvas")}
        >
          <FileUp className="mx-auto h-6 w-6 text-purple-deep" aria-hidden />
          <p className="mt-1 text-sm font-semibold">{t("dropHere")}</p>
          <p className="text-xs text-muted">{t("limits", { types: "PDF, JPG, PNG", mb: MAX_DOC_BYTES / 1024 / 1024 })}</p>
          <input ref={inputRef} id={`file-${category}`} type="file" className="sr-only" accept={ALLOWED_DOC_TYPES.join(",")} onChange={(e) => pick(e.target.files)} />
          <label htmlFor={`file-${category}`} className="mt-3 inline-flex h-9 cursor-pointer items-center rounded-full bg-white px-4 text-[13px] font-semibold shadow-clay-sm focus-within:outline focus-within:outline-2 focus-within:outline-purple hover:-translate-y-0.5">
            {doc.uploaded ? t("replace") : t("choose")}
          </label>
        </div>
      )}

      {showTerminal && (
        <div className="space-y-3">
          <TerminalPanel
            title={state.fileMeta ? `doccheck :: ${state.fileMeta.name}` : "doccheck"}
            lines={state.lines}
            busy={busy}
            indeterminate={state.phase === "processing"}
            progress={state.progress}
            elapsedMs={state.elapsedMs}
          />
          {busy && (
            <div className="flex items-center justify-between gap-3">
              <p className="text-xs text-muted">{state.phase === "processing" ? t("processingNote") : t("uploadingNote")}</p>
              <Button variant="outline" size="sm" onClick={cancel}><X className="h-4 w-4" aria-hidden /> {t("cancel")}</Button>
            </div>
          )}
          {state.phase === "error" && (
            <InlineAlert tone="danger" title={t("failedTitle")}>
              {state.error?.message}
              <div className="mt-2 flex gap-2">
                <Button variant="secondary" size="sm" onClick={reset}><RotateCcw className="h-4 w-4" aria-hidden /> {t("tryAgain")}</Button>
              </div>
            </InlineAlert>
          )}
          {result && (
            <div className="space-y-3 rounded-2xl border-2 border-ink bg-white p-4 shadow-pop">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="flex items-center gap-2"><DocStatusBadge status={result.status} /><AiLabel /></div>
                {result.detected_type && (
                  <p className="text-xs text-muted">
                    {t("detected")}: <strong className="font-mono">{result.detected_type}</strong>
                    {result.confidence != null && <> ({fmt.number(result.confidence, { style: "percent" })})</>}
                  </p>
                )}
              </div>
              {result.status === "NEEDS_HUMAN" && <InlineAlert tone="info">{t("needsHuman")}</InlineAlert>}
              {result.status === "AI_FAIL" && <InlineAlert tone="warning">{t("failNote")}</InlineAlert>}
              <CheckList checks={result.checks} />
              <p className="text-xs text-muted">{t("disclaimer")}</p>
              <Button variant="ghost" size="sm" onClick={reset}>{t("dismiss")}</Button>
            </div>
          )}
        </div>
      )}

      {doc.uploaded && doc.document_id && !busy && (
        <Button variant="ghost" size="sm" onClick={view} loading={opening}><Eye className="h-4 w-4" aria-hidden /> {tc("viewFile")}</Button>
      )}
    </Card>
  );
}
