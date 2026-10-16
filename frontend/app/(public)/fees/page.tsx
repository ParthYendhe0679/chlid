import { getTranslations } from "next-intl/server";
import { PageHead } from "@/components/ui/card";
import { FeeCard } from "@/features/eligibility/fee-card";

export const metadata = { title: "Official Fee Card" };

export default async function Page() {
  const t = await getTranslations("fees");
  return (
    <>
      <PageHead kicker={t("kicker")} title={t("title")} subtitle={t("subtitle")} />
      <FeeCard />
    </>
  );
}
