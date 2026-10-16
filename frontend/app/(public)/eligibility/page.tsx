import { getTranslations } from "next-intl/server";
import { PageHead } from "@/components/ui/card";
import { EligibilityChecker } from "@/features/eligibility/eligibility-checker";

export const metadata = { title: "Eligibility Checker" };

export default async function Page() {
  const t = await getTranslations("eligibility");
  return (
    <>
      <PageHead kicker={t("kicker")} title={t("title")} subtitle={t("subtitle")} />
      <EligibilityChecker />
    </>
  );
}
