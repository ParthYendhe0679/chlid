"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { BadgeCheck, Calculator, Home, IndianRupee, ShieldAlert } from "lucide-react";
import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils/cn";
import { useAuth } from "@/lib/auth/auth-provider";
import { roleHome } from "@/lib/permissions";
import { buttonClass } from "@/components/ui/button";
import { LangSwitcher, Logo } from "./brand";

const items = [
  { href: "/", key: "home", Icon: Home },
  { href: "/eligibility", key: "eligibility", Icon: Calculator },
  { href: "/fees", key: "fees", Icon: IndianRupee },
  { href: "/verify-agency", key: "verifyAgency", Icon: BadgeCheck },
  { href: "/scam-shield", key: "scamShield", Icon: ShieldAlert },
] as const;

export function PublicShell({ children }: { children: React.ReactNode }) {
  const t = useTranslations();
  const pathname = usePathname();
  const { user } = useAuth();

  return (
    <div className="bg-dots min-h-dvh pb-28 md:pb-0">
      <a href="#main" className="sr-only focus:not-sr-only focus:absolute focus:left-3 focus:top-3 focus:z-50 focus:rounded-full focus:bg-white focus:px-4 focus:py-2">
        {t("common.skipToContent")}
      </a>
      <header className="sticky top-0 z-40 border-b border-ink/5 bg-canvas/85 backdrop-blur-md">
        <div className="mx-auto flex h-16 max-w-6xl items-center justify-between gap-3 px-4 sm:px-6">
          <Logo />
          <nav aria-label={t("common.mainNav")} className="hidden items-center gap-1 rounded-full bg-white p-1 shadow-clay-sm md:flex">
            {items.map(({ href, key, Icon }) => {
              const active = pathname === href;
              return (
                <Link
                  key={href}
                  href={href}
                  aria-current={active ? "page" : undefined}
                  className={cn("inline-flex h-10 items-center gap-2 rounded-full px-3.5 text-sm font-semibold transition-colors", active ? "bg-pill text-white" : "text-ink-soft hover:text-ink")}
                >
                  <Icon className="h-[18px] w-[18px]" aria-hidden />
                  {t(`nav.public.${key}`)}
                </Link>
              );
            })}
          </nav>
          <div className="flex items-center gap-2">
            <LangSwitcher />
            {user ? (
              <Link href={roleHome[user.role]} className={buttonClass("primary", "sm")}>{t("common.myArea")}</Link>
            ) : (
              <>
                <Link href="/login" className={cn(buttonClass("ghost", "sm"), "hidden sm:inline-flex")}>{t("common.login")}</Link>
                <Link href="/register" className={buttonClass("primary", "sm")}>{t("common.register")}</Link>
              </>
            )}
          </div>
        </div>
      </header>

      <main id="main" className="mx-auto max-w-6xl px-4 py-8 sm:px-6">{children}</main>

      <footer className="mt-10 bg-ink px-4 py-10 text-white">
        <div className="mx-auto max-w-6xl space-y-2 text-sm">
          <p className="font-display text-lg font-bold">Child Connect</p>
          <p className="max-w-2xl text-white/80">{t("footer.line1")}</p>
          <p className="max-w-2xl text-white/60">{t("footer.line2")}</p>
        </div>
      </footer>

      <nav aria-label={t("common.mainNav")} className="fixed inset-x-3 bottom-3 z-40 rounded-[28px] border-2 border-ink bg-white/95 px-1 py-1.5 shadow-pop backdrop-blur md:hidden">
        <ul className="flex items-center justify-around">
          {items.map(({ href, key, Icon }) => {
            const active = pathname === href;
            return (
              <li key={href}>
                <Link href={href} aria-current={active ? "page" : undefined} className="flex min-w-14 flex-col items-center gap-0.5 rounded-2xl px-1.5 py-1">
                  <span className={cn("grid h-9 w-12 place-items-center rounded-full", active ? "bg-pill text-white" : "text-ink-soft")}><Icon className="h-5 w-5" aria-hidden /></span>
                  <span className={cn("text-[10px] font-semibold", active ? "text-ink" : "text-muted")}>{t(`nav.public.${key}`)}</span>
                </Link>
              </li>
            );
          })}
        </ul>
      </nav>
    </div>
  );
}
