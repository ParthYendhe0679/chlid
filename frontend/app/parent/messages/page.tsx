"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { MessageCircle } from "lucide-react";
import { PageHead } from "@/components/ui/card";
import { buttonClass } from "@/components/ui/button";
import { EmptyState, ErrorState, PageSkeleton } from "@/components/ui/feedback";
import { MessageThread } from "@/features/messages/message-thread";
import { useMyApplication } from "@/features/case-tracker/queries";

export default function Page() {
  const t = useTranslations("messages");
  const app = useMyApplication();
  return (
    <>
      <PageHead kicker={t("kicker")} title={t("title")} subtitle={t("subtitle")} />
      {app.isPending && <PageSkeleton />}
      {app.isError && <ErrorState error={app.error} onRetry={() => app.refetch()} />}
      {app.isSuccess && app.data === null && (
        <EmptyState icon={MessageCircle} title={t("noCaseTitle")} body={t("noCaseBody")} action={<Link href="/parent/profile" className={buttonClass("primary")}>{t("createProfile")}</Link>} />
      )}
      {app.isSuccess && app.data && <MessageThread applicationId={app.data.id} viewerRole="PARENT" />}
    </>
  );
}
