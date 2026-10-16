"use client";

import * as React from "react";
import Link from "next/link";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CalendarClock, FileCheck2, Send } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { Button, buttonClass } from "@/components/ui/button";
import { PageHead } from "@/components/ui/card";
import { EmptyState, ErrorState, InlineAlert, PageSkeleton, StatCard } from "@/components/ui/feedback";
import { Sheet } from "@/components/ui/sheet";
import { useToast } from "@/components/ui/toast";
import { applicationsApi } from "@/lib/api/applications.api";
import { formatDate } from "@/lib/utils/format";
import { qk, useMyApplication } from "@/features/case-tracker/queries";
import { useNow } from "@/features/case-tracker/use-ticker";
import { UploadCard } from "./upload-card";

const UPLOADABLE = new Set(["REGISTERED", "DOCS_CORRECTION"]);

export function DocumentsPage() {
  const t = useTranslations("documents");
  const locale = useLocale();
  const toast = useToast();
  const qc = useQueryClient();
  const q = useMyApplication();
  const now = useNow(60_000);
  const [confirm, setConfirm] = React.useState(false);

  const submit = useMutation({
    mutationFn: applicationsApi.submit,
    onSuccess: () => {
      setConfirm(false);
      toast({ tone: "success", title: t("submitted") });
      qc.invalidateQueries({ queryKey: qk.application });
      qc.invalidateQueries({ queryKey: qk.tracker });
    },
    onError: (e) => { setConfirm(false); toast({ tone: "error", title: t("submitFailed"), body: e.message }); },
  });

  if (q.isPending) return <PageSkeleton />;
  if (q.isError) return <ErrorState error={q.error} onRetry={() => q.refetch()} />;
  if (q.data === null) {
    return (
      <>
        <PageHead title={t("title")} />
        <EmptyState icon={FileCheck2} title={t("needProfileTitle")} body={t("needProfileBody")} action={<Link href="/parent/profile" className={buttonClass("primary")}>{t("createProfile")}</Link>} />
      </>
    );
  }

  const app = q.data;
  const canUpload = UPLOADABLE.has(app.status);
  const docs = app.required_docs;
  const missing = docs.filter((d) => !d.uploaded);
  const withIssues = docs.filter((d) => d.status === "AI_WARN" || d.status === "AI_FAIL" || d.status === "NEEDS_HUMAN");
  const daysLeft = app.doc_deadline ? Math.ceil((new Date(app.doc_deadline).getTime() - now) / 86_400_000) : null;
  const urgent = daysLeft !== null && daysLeft <= 10;
  const uploaded = docs.length - missing.length;

  return (
    <>
      <PageHead kicker={t("kicker")} title={t("title")} subtitle={t("subtitle")}>
        {canUpload && (
          <Button size="md" disabled={missing.length > 0 || docs.length === 0} onClick={() => setConfirm(true)}>
            <Send className="h-4 w-4" aria-hidden /> {t("submit")}
          </Button>
        )}
      </PageHead>

      <div className="mb-6 grid gap-4 sm:grid-cols-2">
        <StatCard
          label={t("countdown")}
          icon={CalendarClock}
          tone={urgent ? "coral" : "sun"}
          value={daysLeft === null ? "-" : <span className="tabular">{Math.max(daysLeft, 0)}</span>}
          hint={app.doc_deadline ? t("deadlineHint", { date: formatDate(app.doc_deadline, locale) }) : t("noDeadline")}
        />
        <StatCard label={t("progress")} icon={FileCheck2} tone="mint" value={`${uploaded} / ${docs.length}`} hint={missing.length ? t("missing", { count: missing.length }) : t("allUploaded")} />
      </div>

      {!canUpload && <InlineAlert tone="info" className="mb-5">{t("locked", { status: app.status })}</InlineAlert>}
      {app.status === "LAPSED" && <InlineAlert tone="danger" className="mb-5" title={t("lapsedTitle")}>{t("lapsedBody")}</InlineAlert>}
      {canUpload && <InlineAlert tone="purple" className="mb-5" title={t("warnOkTitle")}>{t("warnOkBody")}</InlineAlert>}

      {docs.length === 0 ? (
        <EmptyState title={t("emptyTitle")} body={t("emptyBody")} />
      ) : (
        <ul className="grid gap-5 lg:grid-cols-2">
          {docs.map((d) => (
            <li key={d.category}><UploadCard doc={d} canUpload={canUpload} /></li>
          ))}
        </ul>
      )}

      <Sheet
        open={confirm}
        onClose={() => !submit.isPending && setConfirm(false)}
        title={t("confirmTitle")}
        footer={
          <>
            <Button variant="secondary" onClick={() => setConfirm(false)} disabled={submit.isPending}>{t("cancel")}</Button>
            <Button onClick={() => submit.mutate()} loading={submit.isPending}>{t("confirmSubmit")}</Button>
          </>
        }
      >
        <p>{t("confirmBody")}</p>
        {withIssues.length > 0 && (
          <InlineAlert tone="warning" className="mt-3" title={t("confirmIssues", { count: withIssues.length })}>{t("confirmIssuesBody")}</InlineAlert>
        )}
      </Sheet>
    </>
  );
}
