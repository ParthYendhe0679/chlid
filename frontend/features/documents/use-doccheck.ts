"use client";

import * as React from "react";
import { useQueryClient } from "@tanstack/react-query";
import { applicationsApi } from "@/lib/api/applications.api";
import { ApiError } from "@/lib/api/errors";
import { validateDocFile } from "@/lib/validation/schemas";
import { qk } from "@/features/case-tracker/queries";
import type { DocCategory, UploadResult } from "@/types";
import type { TermLine, TermTag } from "./terminal-panel";

export type RunPhase = "idle" | "uploading" | "processing" | "done" | "error";

export interface DocCheckState {
  phase: RunPhase;
  lines: TermLine[];
  progress: number | null;
  elapsedMs: number;
  result: UploadResult | null;
  error: ApiError | Error | null;
  fileMeta: { name: string; size: number; type: string } | null;
}

const initial: DocCheckState = { phase: "idle", lines: [], progress: null, elapsedMs: 0, result: null, error: null, fileMeta: null };

/** Messages are passed as translators so the hook stays UI-agnostic. */
export interface RunText {
  localOk: (m: { type: string; size: string }) => string;
  localBad: (reason: "type" | "size" | "empty") => string;
  uploadStart: string;
  uploadPct: (pct: number) => string;
  uploadSent: string;
  serverWaiting: string;
  check: (c: { code: string; message: string }) => string;
  resultLine: (m: { status: string; type?: string | null; confidence?: number | null }) => string;
  failed: (msg: string) => string;
  cancelled: string;
}

/**
 * Drives one DocCheck run. The backend contract (guide 15.4) is a single synchronous multipart request,
 * so the only real signals are: local validation, upload progress, and the final response.
 * Between "upload sent" and "response" the processing state is shown as indeterminate with real elapsed time.
 */
export function useDocCheck(category: DocCategory, text: RunText) {
  const qc = useQueryClient();
  const [state, setState] = React.useState<DocCheckState>(initial);
  const ctrl = React.useRef<AbortController | null>(null);
  const startedAt = React.useRef(0);
  const lineId = React.useRef(0);
  const inFlight = React.useRef(false);
  const textRef = React.useRef(text);
  React.useEffect(() => { textRef.current = text; });

  // Real elapsed time while a run is active.
  React.useEffect(() => {
    if (state.phase !== "uploading" && state.phase !== "processing") return;
    const id = setInterval(() => setState((s) => ({ ...s, elapsedMs: performance.now() - startedAt.current })), 100);
    return () => clearInterval(id);
  }, [state.phase]);

  const push = React.useCallback((tag: TermTag, msg: string, tone?: TermLine["tone"]) => {
    setState((s) => ({ ...s, lines: [...s.lines, { id: ++lineId.current, tag, text: msg, at: Date.now(), tone }] }));
  }, []);

  const run = React.useCallback(async (file: File) => {
    if (inFlight.current) return; // prevents duplicate submissions
    const tx = textRef.current;
    const meta = { name: file.name, size: file.size, type: file.type || "unknown" };
    const bad = validateDocFile(file);
    setState({ ...initial, fileMeta: meta, phase: bad ? "error" : "uploading", lines: [] });
    startedAt.current = performance.now();
    if (bad) {
      push("ERROR", tx.localBad(bad), "fail");
      setState((s) => ({ ...s, error: new Error(tx.localBad(bad)) }));
      return;
    }
    inFlight.current = true;
    const ac = new AbortController();
    ctrl.current = ac;
    push("LOCAL", tx.localOk({ type: meta.type, size: `${(file.size / 1024).toFixed(0)} KB` }), "ok");
    push("UPLOAD", tx.uploadStart);
    let lastPct = 0;
    let sent = false;
    try {
      const result = await applicationsApi.uploadDocument(category, file, {
        signal: ac.signal,
        onProgress: (f) => {
          setState((s) => ({ ...s, progress: f }));
          const pct = Math.floor(f * 100);
          if (pct >= 100) {
            if (!sent) {
              sent = true;
              push("UPLOAD", tx.uploadSent);
              push("SERVER", tx.serverWaiting);
              setState((s) => ({ ...s, phase: "processing" }));
            }
          } else if (pct >= lastPct + 25) {
            lastPct = pct - (pct % 25);
            push("UPLOAD", tx.uploadPct(pct));
          }
        },
      });
      for (const c of result.checks) push("CHECK", tx.check(c), c.result === "PASS" ? "ok" : c.result === "WARN" ? "warn" : "fail");
      push("RESULT", tx.resultLine({ status: result.status, type: result.detected_type, confidence: result.confidence }),
        result.status === "AI_PASS" || result.status === "VERIFIED" ? "ok" : result.status === "AI_FAIL" ? "fail" : "warn");
      setState((s) => ({ ...s, phase: "done", result, progress: 1, elapsedMs: performance.now() - startedAt.current }));
      qc.invalidateQueries({ queryKey: qk.application });
      qc.invalidateQueries({ queryKey: qk.tracker });
    } catch (e) {
      const err = e as ApiError;
      if (err instanceof ApiError && err.code === "ABORTED") push("ERROR", tx.cancelled, "warn");
      else push("ERROR", tx.failed(err.message), "fail");
      setState((s) => ({ ...s, phase: "error", error: err, elapsedMs: performance.now() - startedAt.current }));
    } finally {
      inFlight.current = false;
      ctrl.current = null;
    }
  }, [category, push, qc]);

  const cancel = React.useCallback(() => ctrl.current?.abort(), []);
  const reset = React.useCallback(() => { if (!inFlight.current) setState(initial); }, []);

  return { state, run, cancel, reset, busy: state.phase === "uploading" || state.phase === "processing" };
}
