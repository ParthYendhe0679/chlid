"use client";

import * as React from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft } from "lucide-react";
import { useTranslations } from "next-intl";
import { PageHead } from "@/components/ui/card";
import { Badge, ErrorState, PageSkeleton } from "@/components/ui/feedback";
import { Tabs, TabPanel } from "@/components/ui/tabs";
import { agencyApi } from "@/lib/api/agency.api";
import { DeadlineBadge } from "@/features/case-tracker/deadline-badge";
import { RiskReasons } from "@/features/deadlines/risk-reasons";
import { MessageThread } from "@/features/messages/message-thread";
import { DocumentReview } from "./document-review";
import { HsrCopilot } from "./hsr-copilot";
import { LegalTimeline } from "./legal-timeline";
import { FollowupReports } from "./followup-reports";

export function CaseDetailPage() {
  const { id } = useParams<{ id: string }>();
  const t = useTranslations("agency");
  const ts = useTranslations("status");
  const [tab, setTab] = React.useState("documents");
  const q = useQuery({ queryKey: ["agency", "case", id], queryFn: ({ signal }) => agencyApi.caseDetail(id, signal) });

  if (q.isPending) return <PageSkeleton />;
  if (q.isError) return <ErrorState error={q.error} onRetry={() => q.refetch()} />;
  const d = q.data;

  return (
    <>
      <Link href="/agency" className="mb-4 inline-flex items-center gap-1 text-sm font-semibold text-purple-deep"><ArrowLeft className="h-4 w-4" aria-hidden /> {t("backToCases")}</Link>
      <PageHead title={d.parent_name} subtitle={t("caseSubtitle")}>
        <Badge tone="purple">{ts(d.status)}</Badge>
        {d.current_deadline && <DeadlineBadge deadline={d.current_deadline} />}
      </PageHead>
      {d.current_deadline && <div className="mb-5"><RiskReasons score={d.current_deadline.risk_score} reasons={d.current_deadline.risk_reasons} /></div>}

      <Tabs
        label={t("caseTabs")}
        active={tab}
        onChange={setTab}
        tabs={[
          { key: "documents", label: t("tabs.documents") },
          { key: "hsr", label: t("tabs.hsr") },
          { key: "timeline", label: t("tabs.timeline") },
          { key: "messages", label: t("tabs.messages") },
          { key: "followups", label: t("tabs.followups") },
        ]}
      />
      <TabPanel id="documents" active={tab}><DocumentReview detail={d} /></TabPanel>
      <TabPanel id="hsr" active={tab}><HsrCopilot key={d.hsr?.signed_at ?? "draft"} detail={d} /></TabPanel>
      <TabPanel id="timeline" active={tab}><LegalTimeline detail={d} /></TabPanel>
      <TabPanel id="messages" active={tab}><MessageThread applicationId={d.application_id} viewerRole="AGENCY" /></TabPanel>
      <TabPanel id="followups" active={tab}><FollowupReports detail={d} /></TabPanel>
    </>
  );
}
