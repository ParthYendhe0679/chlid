import { getTranslations } from "next-intl/server";
import { PageHead } from "@/components/ui/card";
import { VerifyAgency } from "@/features/scam-shield/verify-agency";

export const metadata = { title: "Verify Agency" };

export default async function Page() {
  const t = await getTranslations("verify");
  return (
    <>
      <PageHead kicker={t("kicker")} title={t("title")} subtitle={t("subtitle")} />
      <VerifyAgency />
    </>
  );
}
