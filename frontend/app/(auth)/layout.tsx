import Link from "next/link";
import { getTranslations } from "next-intl/server";
import { BadgeCheck, FileCheck2, HeartHandshake, Languages } from "lucide-react";
import { LangSwitcher } from "@/components/layout/brand";

const lobster = { fontFamily: "var(--font-lobster), cursive" } as const;

export default async function AuthLayout({ children }: { children: React.ReactNode }) {
  const t = await getTranslations("auth");
  const points = [
    { Icon: FileCheck2, text: t("point1") },
    { Icon: Languages, text: t("point2") },
    { Icon: BadgeCheck, text: t("point3") },
  ];
  return (
    <div className="grid min-h-dvh bg-[#fff6f3] lg:grid-cols-[1.05fr_1fr]">
      <aside
        className="relative hidden flex-col justify-between overflow-hidden p-10 text-white lg:flex"
        style={{ backgroundImage: "linear-gradient(0deg,rgba(0,0,0,.4),rgba(0,0,0,.05) 60%), url(/hero-1.jpg), linear-gradient(135deg,#f4573b 0%,#f08a6c 50%,#fad47f 100%)", backgroundSize: "cover", backgroundPosition: "center" }}
      >
        <Link href="/" className="flex items-center gap-2 text-white" style={lobster}>
          <span className="grid h-11 w-11 place-items-center rounded-full bg-white text-[#f4573b]"><HeartHandshake className="h-6 w-6" aria-hidden /></span>
          <span className="text-3xl font-bold">Child Connect</span>
        </Link>
        <div className="space-y-5">
          <h1 className="max-w-md text-6xl font-bold leading-[1.1] drop-shadow" style={lobster}>{t("billboard")}</h1>
          <ul className="space-y-3">
            {points.map(({ Icon, text }) => (
              <li key={text} className="flex max-w-sm items-center gap-3 rounded-full bg-white/90 px-4 py-3 text-[#1f2d3a] shadow-lg">
                <Icon className="h-5 w-5 shrink-0 text-[#f4573b]" aria-hidden />
                <span className="text-sm font-semibold">{text}</span>
              </li>
            ))}
          </ul>
        </div>
        <p className="max-w-md text-xs font-medium text-white/90">{t("principle")}</p>
        {/* scalloped right edge, echoing the landing header */}
        <div aria-hidden className="absolute inset-y-0 -right-px w-[14px]" style={{ background: "radial-gradient(circle at 14px 14px, #fff6f3 13px, transparent 14px) 0 0 / 14px 28px repeat-y" }} />
      </aside>
      <main id="main" className="flex flex-col px-4 py-6 sm:px-8">
        <div className="mb-6 flex items-center justify-between lg:justify-end">
          <Link href="/" className="text-3xl font-bold text-[#f4573b] lg:hidden" style={lobster}>Child Connect</Link>
          <LangSwitcher />
        </div>
        <div className="mx-auto my-auto w-full max-w-md rounded-[28px] bg-white p-6 shadow-[0_20px_50px_-24px_rgba(244,87,59,.5)] sm:p-8 [&_h1]:font-[family-name:var(--font-lobster)] [&_h1]:text-4xl [&_h1]:font-bold [&_h1]:text-[#1f2d3a]">{children}</div>
      </main>
    </div>
  );
}
