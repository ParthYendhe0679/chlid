import { getTranslations } from "next-intl/server";
import { PageHead } from "@/components/ui/card";
import { ScamShield } from "@/features/scam-shield/scam-shield";

export const metadata = { title: "Scam Shield" };

export default async function Page() {
  const t = await getTranslations("scam");
  return (
    <>
      <PageHead kicker={t("kicker")} title={t("title")} subtitle={t("subtitle")} />
      <ScamShield />
    </>
  );
}
