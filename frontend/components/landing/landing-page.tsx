"use client";

import * as React from "react";
import Link from "next/link";
import dynamic from "next/dynamic";
import { ArrowRight, BadgeCheck, Bot, Calculator, ChevronLeft, ChevronRight, FileCheck2, HeartHandshake, IndianRupee, Mail, ShieldAlert, ShieldCheck } from "lucide-react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/lib/auth/auth-provider";
import { roleHome } from "@/lib/permissions";
import { LangSwitcher } from "@/components/layout/brand";
import { cn } from "@/lib/utils/cn";

const ThreeScene = dynamic(() => import("./three-scene").then((m) => m.ThreeScene), { ssr: false });

const NAV = [
  { href: "#home", key: "home" },
  { href: "#about", key: "about" },
  { href: "#mission", key: "mission" },
  { href: "#adopt", key: "adopt" },
  { href: "#contact", key: "contact" },
  { href: "/parent/assistant", key: "mentor" },
] as const;

const SLIDES = ["s1", "s2", "s3"] as const;
const slideBg = [
  "linear-gradient(135deg,#f4573b 0%,#f08a6c 45%,#fad47f 100%)",
  "linear-gradient(135deg,#1f4e5a 0%,#2f6fbf 60%,#c4def8 100%)",
  "linear-gradient(135deg,#7b5cf0 0%,#c6b5f6 55%,#fde4e2 100%)",
];

const lobster = { fontFamily: "var(--font-lobster), cursive" } as const;

export function LandingPage() {
  const t = useTranslations("landing");
  const { user } = useAuth();
  const [i, setI] = React.useState(0);

  React.useEffect(() => {
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    const id = setInterval(() => setI((x) => (x + 1) % SLIDES.length), 7000);
    return () => clearInterval(id);
  }, []);

  const tools = [
    { href: "/eligibility", key: "eligibility", Icon: Calculator },
    { href: "/fees", key: "fees", Icon: IndianRupee },
    { href: "/verify-agency", key: "verify", Icon: BadgeCheck },
    { href: "/scam-shield", key: "scam", Icon: ShieldAlert },
  ] as const;
  const feats = [
    { key: "doccheck", Icon: FileCheck2 }, { key: "sahayak", Icon: Bot }, { key: "tracker", Icon: HeartHandshake }, { key: "privacy", Icon: ShieldCheck },
  ] as const;

  return (
    <div className="relative min-h-dvh bg-[#fff6f3]">
      <ThreeScene />
      <a href="#main" className="sr-only focus:not-sr-only focus:absolute focus:left-3 focus:top-3 focus:z-50 focus:rounded-full focus:bg-white focus:px-4 focus:py-2">{t("skip")}</a>

      <div className="relative z-10 mx-auto max-w-[1200px] px-3 sm:px-6">
        {/* Header with scalloped lower edge, as in the reference */}
        <header className="sticky top-0 z-40">
          <div className="relative rounded-t-[28px] bg-white shadow-[0_12px_30px_-18px_rgba(0,0,0,.25)]">
            <div className="flex h-20 items-center justify-between gap-3 px-4 sm:px-8">
              <Link href="/" className="flex items-center gap-2 text-[#f4573b]" style={lobster}>
                <HeartHandshake className="h-9 w-9" aria-hidden />
                <span className="whitespace-nowrap text-3xl font-bold">Child Connect</span>
              </Link>
              <nav aria-label={t("nav")} className="hidden items-center gap-6 whitespace-nowrap text-[15px] font-medium text-[#1f2d3a] lg:flex">
                {NAV.map((n, idx) => (
                  <a key={n.key} href={n.href} className={cn("transition-colors hover:text-[#f4573b]", idx === 0 && "text-[#f4573b]")}>{t(`navItems.${n.key}`)}</a>
                ))}
              </nav>
              <div className="flex items-center gap-2">
                <LangSwitcher />
                {user ? (
                  <Link href={roleHome[user.role]} className="inline-flex h-11 items-center gap-2 rounded-full bg-[#f4573b] px-5 text-sm font-bold text-white hover:brightness-105">{t("myArea")} <ArrowRight className="h-4 w-4" aria-hidden /></Link>
                ) : (
                  <>
                    <Link href="/login" className="hidden h-11 items-center whitespace-nowrap rounded-full px-4 text-sm font-semibold text-[#1f2d3a] hover:text-[#f4573b] sm:inline-flex">{t("login")}</Link>
                    <Link href="/register" className="inline-flex h-11 items-center gap-2 rounded-full bg-[#f4573b] px-5 text-sm font-bold text-white hover:brightness-105">{t("register")} <ArrowRight className="h-4 w-4" aria-hidden /></Link>
                  </>
                )}
              </div>
            </div>
            <div aria-hidden className="absolute inset-x-0 -bottom-[14px] h-[14px]" style={{ background: "radial-gradient(circle at 14px 0, #fff 13px, transparent 14px) 0 0 / 28px 14px repeat-x" }} />
          </div>
          <nav aria-label={t("nav")} className="mt-5 flex gap-4 overflow-x-auto rounded-full bg-white/90 px-4 py-2 text-sm font-medium lg:hidden">
            {NAV.map((n) => <a key={n.key} href={n.href} className="shrink-0 hover:text-[#f4573b]">{t(`navItems.${n.key}`)}</a>)}
          </nav>
        </header>

        <main id="main">
          {/* Hero slider */}
          <section id="home" aria-roledescription="carousel" aria-label={t("slider")} className="relative mt-6 min-h-[34rem] overflow-hidden rounded-[32px] bg-[#fff6f3] shadow-[0_30px_60px_-30px_rgba(244,87,59,.55)] ring-1 ring-black/5 sm:min-h-[40rem]">
            {SLIDES.map((s, idx) => (
              <div
                key={s}
                aria-hidden={idx !== i}
                className={cn("absolute inset-0 flex flex-col justify-end bg-cover bg-center px-6 pb-16 transition-opacity duration-700 sm:px-10", idx === i ? "opacity-100" : "pointer-events-none opacity-0")}
                style={{ backgroundImage: `linear-gradient(0deg,rgba(0,0,0,.45),rgba(0,0,0,.05) 60%), url(/hero-${idx + 1}.jpg), url(/hero-1.jpg), ${slideBg[idx]}` }}
              >
                <h1 className="max-w-3xl text-5xl font-bold leading-[1.1] text-white drop-shadow sm:text-7xl" style={lobster}>{t(`slides.${s}.title`)}</h1>
                <p className="mt-5 max-w-2xl text-lg font-medium text-white sm:text-xl">{t(`slides.${s}.body`)}</p>
                <div className="mt-8 flex flex-wrap gap-4">
                  <a href="#about" className="inline-flex h-14 items-center rounded-full bg-[#f4573b] px-8 text-base font-bold text-white hover:brightness-105">{t("learnMore")}</a>
                  <Link href="/eligibility" className="inline-flex h-14 items-center rounded-full bg-[#1f4e5a] px-8 text-base font-bold text-white hover:brightness-110">{t("startAdoption")}</Link>
                </div>
              </div>
            ))}
            <div className="absolute right-6 top-1/2 flex -translate-y-1/2 flex-col gap-4">
              <button type="button" aria-label={t("prev")} onClick={() => setI((i + SLIDES.length - 1) % SLIDES.length)} className="grid h-12 w-12 place-items-center rounded-full border-2 border-white/80 text-white backdrop-blur hover:bg-white/20"><ChevronLeft aria-hidden /></button>
              <button type="button" aria-label={t("next")} onClick={() => setI((i + 1) % SLIDES.length)} className="grid h-12 w-12 place-items-center rounded-full border-2 border-white/80 text-white backdrop-blur hover:bg-white/20"><ChevronRight aria-hidden /></button>
            </div>
            <p className="sr-only" aria-live="polite">{t("slideOf", { n: i + 1, total: SLIDES.length })}</p>
          </section>

          <Section id="about" title={t("about.title")} body={t("about.body")}>
            <ul className="mt-8 grid gap-5 sm:grid-cols-2">
              {feats.map(({ key, Icon }) => (
                <li key={key} className="rounded-3xl bg-white p-6 shadow-[0_10px_30px_-14px_rgba(244,87,59,.35)]">
                  <Icon className="h-8 w-8 text-[#f4573b]" aria-hidden />
                  <h3 className="mt-3 text-lg font-bold">{t(`features.${key}.title`)}</h3>
                  <p className="mt-1 text-sm text-[#3a3850]">{t(`features.${key}.body`)}</p>
                </li>
              ))}
            </ul>
          </Section>

          <Section id="mission" title={t("mission.title")} body={t("mission.body")}>
            <ul className="mt-8 grid gap-4 sm:grid-cols-3">
              {(["p1", "p2", "p3"] as const).map((k) => <li key={k} className="rounded-3xl bg-[#1f4e5a] p-6 text-white"><ShieldCheck className="mb-2 h-6 w-6 text-[#fad47f]" aria-hidden />{t(`mission.${k}`)}</li>)}
            </ul>
          </Section>

          <Section id="adopt" title={t("adopt.title")} body={t("adopt.body")}>
            <ul className="mt-8 grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
              {tools.map(({ href, key, Icon }) => (
                <li key={key}>
                  <Link href={href} className="group block h-full rounded-3xl bg-white p-6 shadow-[0_10px_30px_-14px_rgba(244,87,59,.35)] transition hover:-translate-y-1">
                    <Icon className="h-8 w-8 text-[#f4573b]" aria-hidden />
                    <h3 className="mt-3 text-lg font-bold">{t(`tools.${key}.title`)}</h3>
                    <p className="mt-1 text-sm text-[#3a3850]">{t(`tools.${key}.body`)}</p>
                    <span className="mt-3 inline-flex items-center gap-1 text-sm font-bold text-[#f4573b]">{t("open")} <ArrowRight className="h-4 w-4 transition group-hover:translate-x-1" aria-hidden /></span>
                  </Link>
                </li>
              ))}
            </ul>
            <p className="mt-6 text-center"><Link href="/guidance" className="font-bold text-[#f4573b] underline">{t("howItWorks")}</Link></p>
          </Section>

          <Section id="contact" title={t("contact.title")} body={t("contact.body")}>
            <div className="mt-6 flex flex-wrap items-center justify-center gap-4">
              <Link href="/register" className="inline-flex h-14 items-center rounded-full bg-[#f4573b] px-8 font-bold text-white">{t("register")}</Link>
              <Link href="/parent/assistant" className="inline-flex h-14 items-center gap-2 rounded-full bg-[#1f4e5a] px-8 font-bold text-white"><Mail className="h-5 w-5" aria-hidden /> {t("askSahayak")}</Link>
            </div>
            <p className="mt-6 text-center text-sm text-[#3a3850]">{t("contact.helpline")}</p>
          </Section>
        </main>

        <footer className="mb-6 mt-10 rounded-3xl bg-[#1f2d3a] p-8 text-sm text-white">
          <p className="text-2xl font-bold text-[#f4573b]" style={lobster}>Child Connect</p>
          <p className="mt-2 max-w-2xl text-white/80">{t("footer1")}</p>
          <p className="mt-1 max-w-2xl text-white/60">{t("footer2")}</p>
        </footer>
      </div>
    </div>
  );
}

function Section({ id, title, body, children }: { id: string; title: string; body: string; children: React.ReactNode }) {
  return (
    <section id={id} className="scroll-mt-28 py-16 text-center">
      <h2 className="text-4xl font-bold text-[#1f2d3a] sm:text-5xl" style={lobster}>{title}</h2>
      <p className="mx-auto mt-4 max-w-2xl text-[17px] leading-relaxed text-[#3a3850]">{body}</p>
      <div className="text-left">{children}</div>
    </section>
  );
}
