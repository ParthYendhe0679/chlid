"use client";

import * as React from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { ChevronRight, FolderOpen } from "lucide-react";
import { useTranslations } from "next-intl";
import { Card, PageHead } from "@/components/ui/card";
import { Select } from "@/components/ui/form";
import { Badge, EmptyState, ErrorState, Skeleton } from "@/components/ui/feedback";
import { agencyApi } from "@/lib/api/agency.api";
import { DeadlineBadge } from "@/features/case-tracker/deadline-badge";
import { RiskReasons } from "@/features/deadlines/risk-reasons";
import type { AppStatus } from "@/types";

const STATUSES: AppStatus[] = ["DOCS_SUBMITTED", "DOCS_CORRECTION", "HSR_IN_PROGRESS", "WAITING", "REFERRED", "RESERVED", "MATCHED_FOSTER_CARE", "DM_ORDER_PENDING", "ADOPTED_FOLLOWUP", "COMPLETED"];

export function CaseList() {
  const t = useTranslations("agency");
  const ts = useTranslations("status");
  const [status, setStatus] = React.useState("");
  const q = useQuery({ queryKey: ["agency", "cases", status], queryFn: ({ signal }) => agencyApi.cases({ status: status || undefined, sort: "risk" }, signal) });

  return (
    <>
      <PageHead kicker={t("kicker")} title={t("casesTitle")} subtitle={t("casesSubtitle")}>
        <label className="sr-only" htmlFor="status-filter">{t("filterStatus")}</label>
        <Select id="status-filter" value={status} onChange={(e) => setStatus(e.target.value)} className="w-56 rounded-full">
          <option value="">{t("allStatuses")}</option>
          {STATUSES.map((s) => <option key={s} value={s}>{ts(s)}</option>)}
        </Select>
      </PageHead>

      {q.isPending && <div className="space-y-3"><Skeleton className="h-20" /><Skeleton className="h-20" /><Skeleton className="h-20" /></div>}
      {q.isError && <ErrorState error={q.error} onRetry={() => q.refetch()} />}
      {q.isSuccess && q.data.length === 0 && <EmptyState icon={FolderOpen} title={t("noCasesTitle")} body={t("noCasesBody")} />}
      {q.isSuccess && q.data.length > 0 && (
        <ul className="space-y-3" aria-live="polite">
          {q.data.map((c) => (
            <li key={c.application_id}>
              <Link href={`/agency/cases/${c.application_id}`} className="block">
                <Card variant="bento" interactive className="flex flex-wrap items-center justify-between gap-3">
                  <div className="min-w-0">
                    <p className="font-bold">{c.parent_name}</p>
                    <div className="mt-1 flex flex-wrap items-center gap-2">
                      <Badge tone="purple">{ts(c.status)}</Badge>
                      {c.current_deadline && <DeadlineBadge deadline={c.current_deadline} />}
                    </div>
                    {c.current_deadline && <div className="mt-2"><RiskReasons score={c.current_deadline.risk_score} reasons={c.current_deadline.risk_reasons} compact /></div>}
                  </div>
                  <ChevronRight className="h-5 w-5 text-muted" aria-hidden />
                </Card>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </>
  );
}
