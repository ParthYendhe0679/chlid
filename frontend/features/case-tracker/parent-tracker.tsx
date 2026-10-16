"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { CalendarClock, FileWarning, HeartHandshake, Hourglass, UserPlus } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { Card, PageHead } from "@/components/ui/card";
import { buttonClass } from "@/components/ui/button";
import { Badge, EmptyState, ErrorState, InlineAlert, PageSkeleton, StatCard } from "@/components/ui/feedback";
import { applicationsApi } from "@/lib/api/applications.api";
import { formatDate } from "@/lib/utils/format";
import { StageStepper } from "./stage-stepper";
import { DeadlineBadge } from "./deadline-badge";
import { qk, useMyApplication, useTracker } from "./queries";
import { FollowupSchedule } from "./followup-schedule";

export function ParentTracker() {
  const t = useTranslations("tracker");
  const ts = useTranslations("status");
  const locale = useLocale();
  const app = useMyApplication();
  const tracker = useTracker(app.isSuccess && app.data !== null);
  const followups = useQuery({
    queryKey: qk.followups,
    queryFn: ({ signal }) => applicationsApi.followups(signal),
    enabled: tracker.data?.status === "ADOPTED_FOLLOWUP" || tracker.data?.status === "COMPLETED",
  });

  if (app.isPending) return <PageSkeleton />;
  if (app.isError) return <ErrorState error={app.error} onRetry={() => app.refetch()} />;
  if (app.data === null) {
    return (
      <>
        <PageHead title={t("title")} subtitle={t("subtitle")} />
        <EmptyState
          icon={UserPlus}
          title={t("noAppTitle")}
          body={t("noAppBody")}
          action={<Link href="/parent/profile" className={buttonClass("primary")}>{t("createProfile")}</Link>}
        />
      </>
    );
  }
  if (tracker.isPending) return <PageSkeleton />;
  if (tracker.isError) return <ErrorState error={tracker.error} onRetry={() => tracker.refetch()} />;

  const tr = tracker.data;
  const corrections = app.data.required_docs.filter((d) => d.status === "REUPLOAD_REQUESTED");
  const status = tr.status;

  return (
    <>
      <PageHead kicker={t("kicker")} title={t("title")} subtitle={t("subtitle")}>
        <Badge tone="purple">{ts(status)}</Badge>
      </PageHead>

      {status === "DEBARRED" && (
        <InlineAlert tone="danger" className="mb-5" title={t("debarredTitle")}>
          {t("debarredBody", { date: formatDate(app.data.debarred_until, locale) })}
        </InlineAlert>
      )}
      {status === "LAPSED" && <InlineAlert tone="danger" className="mb-5" title={t("lapsedTitle")}>{t("lapsedBody")}</InlineAlert>}
      {corrections.length > 0 && (
        <InlineAlert tone="warning" className="mb-5" title={t("correctionsTitle")}>
          <ul className="mt-1 space-y-1">
            {corrections.map((c) => (
              <li key={c.category} className="flex items-start gap-2">
                <FileWarning className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
                <span><strong>{c.category}</strong>{c.reviewer_note ? `: ${c.reviewer_note}` : ""}</span>
              </li>
            ))}
          </ul>
          <Link href="/parent/documents" className={`${buttonClass("outline", "sm")} mt-3`}>{t("goToDocuments")}</Link>
        </InlineAlert>
      )}
      {status === "REFERRED" && (
        <InlineAlert tone="info" className="mb-5" title={t("referralTitle")}>
          <Link href="/parent/referral" className={`${buttonClass("purple", "sm")} mt-2`}>{t("viewReferral")}</Link>
        </InlineAlert>
      )}

      <div className="mb-6 grid gap-4 sm:grid-cols-3">
        <StatCard
          label={t("currentDeadline")}
          icon={CalendarClock}
          tone="sun"
          value={tr.current_deadline ? <span className="tabular">{Math.max(tr.current_deadline.days_left, 0)}</span> : "-"}
          hint={tr.current_deadline ? t("daysLeftHint", { date: formatDate(tr.current_deadline.due_at, locale) }) : t("noDeadline")}
        />
        <StatCard label={t("referralsUsed")} icon={HeartHandshake} tone="coral" value={`${tr.referrals_used} / 3`} hint={t("referralsHint")} />
        <StatCard label={t("seniority")} icon={Hourglass} tone="sky" value={app.data.seniority_date ? formatDate(app.data.seniority_date, locale) : "-"} hint={t("seniorityHint")} />
      </div>

      <div className="grid gap-6 lg:grid-cols-[1.1fr_1fr]">
        <Card>
          <h2 className="mb-4 text-xl font-bold">{t("stages")}</h2>
          <StageStepper stages={tr.stages} currentIndex={tr.stage_index} />
        </Card>
        <div className="space-y-6">
          <Card variant="lavender">
            <h2 className="text-lg font-bold">{t("nextStep")}</h2>
            <p className="mt-2 text-[15px] leading-relaxed">{tr.next_step_text}</p>
            {tr.current_deadline && (
              <div className="mt-4"><DeadlineBadge deadline={tr.current_deadline} /></div>
            )}
          </Card>
          {tr.referrals_used > 0 && tr.referrals_used < 3 && (
            <InlineAlert tone="warning" title={t("rulesTitle")}>
              <ul className="list-disc space-y-1 pl-4">
                <li>{t("rule1")}</li>
                <li>{t("rule2")}</li>
              </ul>
            </InlineAlert>
          )}
          {followups.data && followups.data.length > 0 && <FollowupSchedule items={followups.data} />}
          {followups.isError && <ErrorState compact error={followups.error} onRetry={() => followups.refetch()} />}
          <InlineAlert tone="info">{t("disclaimer")}</InlineAlert>
        </div>
      </div>
    </>
  );
}
