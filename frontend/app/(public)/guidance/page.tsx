import Link from "next/link";
import { getTranslations } from "next-intl/server";
import { Card, PageHead } from "@/components/ui/card";
import { buttonClass } from "@/components/ui/button";
import { InlineAlert } from "@/components/ui/feedback";

export const metadata = { title: "Adoption guidance" };

const STEPS = Array.from({ length: 13 }, (_, i) => `s${i + 1}`);

export default async function Page() {
  const t = await getTranslations("guidance");
  return (
    <>
      <PageHead kicker={t("kicker")} title={t("title")} subtitle={t("subtitle")}>
        <Link href="/eligibility" className={buttonClass("primary")}>{t("cta")}</Link>
      </PageHead>
      <InlineAlert tone="info" className="mb-6" title={t("noteTitle")}>{t("noteBody")}</InlineAlert>
      <ol className="grid gap-4 md:grid-cols-2">
        {STEPS.map((k, i) => (
          <li key={k}>
            <Card className="relative h-full overflow-hidden">
              <span aria-hidden className="absolute right-4 top-1 font-display text-6xl font-extrabold text-ink/5">{String(i + 1).padStart(2, "0")}</span>
              <h2 className="relative text-lg font-bold">{t(`steps.${k}.title`)}</h2>
              <p className="relative mt-2 text-sm text-ink-soft">{t(`steps.${k}.body`)}</p>
              {t.has(`steps.${k}.limit`) && (
                <p className="relative mt-3 inline-block rounded-full bg-sun-soft px-3 py-1 text-xs font-bold text-sun-deep">{t(`steps.${k}.limit`)}</p>
              )}
            </Card>
          </li>
        ))}
      </ol>
    </>
  );
}
