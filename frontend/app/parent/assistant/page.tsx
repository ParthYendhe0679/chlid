import { getTranslations } from "next-intl/server";
import { PageHead } from "@/components/ui/card";
import { AssistantChat } from "@/features/sahayak/assistant-chat";

export const metadata = { title: "Sahayak AI" };

export default async function Page() {
  const t = await getTranslations("assistant");
  return (
    <>
      <PageHead kicker={t("kicker")} title={t("title")} subtitle={t("subtitle")} />
      <AssistantChat />
    </>
  );
}
