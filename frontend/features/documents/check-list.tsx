"use client";

import { AlertTriangle, CheckCircle2, XCircle } from "lucide-react";
import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/feedback";
import type { DocCheck, DocStatus } from "@/types";

const icon = { PASS: CheckCircle2, WARN: AlertTriangle, FAIL: XCircle } as const;
const color = { PASS: "text-success", WARN: "text-warning", FAIL: "text-danger" } as const;

export function CheckList({ checks }: { checks: DocCheck[] }) {
  const t = useTranslations("doccheck");
  if (checks.length === 0) return <p className="text-sm text-muted">{t("noChecks")}</p>;
  return (
    <ul className="space-y-2">
      {checks.map((c) => {
        const Icon = icon[c.result];
        return (
          <li key={c.code} className="flex items-start gap-3 rounded-2xl bg-canvas p-3 text-sm">
            <Icon className={`mt-0.5 h-4 w-4 shrink-0 ${color[c.result]}`} aria-hidden />
            <div>
              <p className="font-semibold">
                <span className="sr-only">{t(`result.${c.result}`)}: </span>
                {c.message}
              </p>
              <p className="font-mono text-[11px] text-muted">{c.code}</p>
            </div>
          </li>
        );
      })}
    </ul>
  );
}

const statusTone: Record<DocStatus, "success" | "warning" | "danger" | "info" | "neutral"> = {
  AI_PASS: "success", AI_WARN: "warning", AI_FAIL: "danger", NEEDS_HUMAN: "info", VERIFIED: "success", REUPLOAD_REQUESTED: "danger",
};

export function DocStatusBadge({ status }: { status: DocStatus }) {
  const t = useTranslations("docStatus");
  return <Badge tone={statusTone[status]}>{t(status)}</Badge>;
}
