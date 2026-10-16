"use client";

import * as React from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { PenLine, Sparkles } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Field, Input, Textarea } from "@/components/ui/form";
import { AiLabel, Badge, InlineAlert, ProgressBar } from "@/components/ui/feedback";
import { Sheet } from "@/components/ui/sheet";
import { useToast } from "@/components/ui/toast";
import { aiApi } from "@/lib/api/ai.api";
import { agencyApi } from "@/lib/api/agency.api";
import { formatDate } from "@/lib/utils/format";
import type { AgencyCaseDetail } from "@/types";

type FU = AgencyCaseDetail["followups"][number];
const SECTIONS = ["child_health", "adjustment", "schooling", "bonding", "concerns"] as const;

export function FollowupReports({ detail }: { detail: AgencyCaseDetail }) {
  const t = useTranslations("followupReport");
  const locale = useLocale();
  const [open, setOpen] = React.useState<FU | null>(null);
  if (detail.followups.length === 0) return <InlineAlert tone="info">{t("none")}</InlineAlert>;
  return (
    <div className="space-y-3">
      {[...detail.followups].sort((a, b) => a.visit_number - b.visit_number).map((f) => (
        <Card key={f.id} variant="bento" className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className="font-bold">{t("visit", { n: f.visit_number })}</p>
            <p className="text-xs text-muted">{t("due", { date: formatDate(f.due_date, locale) })}{f.visited_on ? ` | ${t("visitedOn", { date: formatDate(f.visited_on, locale) })}` : ""}</p>
          </div>
          <div className="flex items-center gap-2">
            <Badge tone={f.visited_on ? "success" : "info"}>{f.visited_on ? t("signedBadge") : t("scheduled")}</Badge>
            {!f.visited_on && <Button size="sm" variant="outline" onClick={() => setOpen(f)}><PenLine className="h-4 w-4" aria-hidden /> {t("write")}</Button>}
          </div>
        </Card>
      ))}
      {open && <FollowupSheet key={open.id} caseId={detail.application_id} fu={open} onClose={() => setOpen(null)} />}
    </div>
  );
}

function FollowupSheet({ caseId, fu, onClose }: { caseId: string; fu: FU; onClose: () => void }) {
  const t = useTranslations("followupReport");
  const toast = useToast();
  const qc = useQueryClient();
  const [notes, setNotes] = React.useState("");
  const [visited, setVisited] = React.useState("");
  const [sections, setSections] = React.useState<Record<string, string>>({});
  const [flags, setFlags] = React.useState<string[]>([]);
  const [drafted, setDrafted] = React.useState(false);

  const draft = useMutation({
    mutationFn: () => aiApi.followupDraft({ followup_id: fu.id, raw_notes: notes }),
    onSuccess: (r) => { setSections(r.sections); setFlags(r.concern_flags); setDrafted(true); toast({ tone: "ai", title: t("draftReady") }); },
    onError: (e) => toast({ tone: "error", title: t("draftFailed"), body: e.message }),
  });
  const sign = useMutation({
    mutationFn: () => agencyApi.signFollowup(fu.id, { final_report: sections, visited_on: visited }),
    onSuccess: () => { toast({ tone: "success", title: t("signed") }); qc.invalidateQueries({ queryKey: ["agency", "case", caseId] }); onClose(); },
    onError: (e) => toast({ tone: "error", title: t("signFailed"), body: e.message }),
  });
  const keys = Array.from(new Set([...SECTIONS, ...Object.keys(sections)]));

  return (
    <Sheet
      wide
      open
      onClose={() => !sign.isPending && onClose()}
      title={t("sheetTitle", { n: fu.visit_number })}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={sign.isPending}>{t("cancel")}</Button>
          <Button onClick={() => sign.mutate()} loading={sign.isPending} disabled={!drafted || !visited}><PenLine className="h-4 w-4" aria-hidden /> {t("sign")}</Button>
        </>
      }
    >
      <div className="space-y-4">
        <Field label={t("visitedLabel")} htmlFor="fu-visited"><Input id="fu-visited" type="date" value={visited} onChange={(e) => setVisited(e.target.value)} /></Field>
        <Field label={t("notes")} htmlFor="fu-notes"><Textarea id="fu-notes" rows={5} value={notes} onChange={(e) => setNotes(e.target.value)} /></Field>
        <Button variant="purple" onClick={() => draft.mutate()} loading={draft.isPending} disabled={notes.trim().length < 20}><Sparkles className="h-4 w-4" aria-hidden /> {t("draftWithAi")}</Button>
        {draft.isPending && <ProgressBar indeterminate label={t("drafting")} />}
        {drafted && (
          <>
            <div className="flex items-center gap-2"><AiLabel /><span className="text-xs text-muted">{t("editBeforeSign")}</span></div>
            {flags.length > 0 && (
              <InlineAlert tone="warning" title={t("concernFlags")}>
                <ul className="list-disc pl-5">{flags.map((f) => <li key={f}>{f}</li>)}</ul>
              </InlineAlert>
            )}
            {keys.map((k) => (
              <Field key={k} label={t.has(`sections.${k}`) ? t(`sections.${k}`) : k} htmlFor={`fu-${k}`}>
                <Textarea id={`fu-${k}`} rows={3} value={sections[k] ?? ""} onChange={(e) => setSections((s) => ({ ...s, [k]: e.target.value }))} />
              </Field>
            ))}
          </>
        )}
      </div>
    </Sheet>
  );
}
