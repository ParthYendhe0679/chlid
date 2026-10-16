"use client";

import * as React from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import {
  Baby, BellRing, BookOpen, Building2, FileText, FolderKanban, HeartHandshake, LogOut, MessageCircle, Radar,
  Route as RouteIcon, ScrollText, ShieldAlert, Sparkles, UserRound,
} from "lucide-react";
import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils/cn";
import { useAuth } from "@/lib/auth/auth-provider";
import { navByRole, requiredRoleFor, roleHome } from "@/lib/permissions";
import { Button, buttonClass } from "@/components/ui/button";
import { PageSkeleton, EmptyState } from "@/components/ui/feedback";
import { LangSwitcher, Logo } from "./brand";
import { NotificationBell } from "./notification-bell";
import type { Role } from "@/types";

const icons = {
  route: RouteIcon, file: FileText, heart: HeartHandshake, message: MessageCircle, sparkles: Sparkles, user: UserRound,
  folder: FolderKanban, baby: Baby, radar: Radar, bell: BellRing, building: Building2, book: BookOpen, shield: ShieldAlert, scroll: ScrollText,
} as const;

const isActive = (pathname: string, href: string, all: string[]) =>
  pathname === href || (pathname.startsWith(href + "/") && !all.some((o) => o !== href && o.length > href.length && (pathname === o || pathname.startsWith(o + "/"))));

/** Role-gated layout. Guards are a usability measure; the server enforces real permissions. */
export function RoleShell({ role, children }: { role: Role; children: React.ReactNode }) {
  const { user, status, logout, sessionExpired } = useAuth();
  const t = useTranslations();
  const pathname = usePathname();
  const router = useRouter();

  React.useEffect(() => {
    if (status === "anon") {
      const q = new URLSearchParams({ next: pathname });
      if (sessionExpired) q.set("expired", "1");
      router.replace(`/login?${q.toString()}`);
    }
  }, [status, pathname, router, sessionExpired]);

  if (status === "loading" || status === "anon") {
    return <div className="mx-auto max-w-6xl px-4 py-10"><PageSkeleton /></div>;
  }
  if (user && user.role !== role) {
    return (
      <div className="bg-dots min-h-dvh px-4 py-20">
        <div className="mx-auto max-w-lg">
          <EmptyState
            icon={ShieldAlert}
            title={t("common.errors.accessDeniedTitle")}
            body={t("common.errors.accessDeniedBody")}
            action={<Link href={roleHome[user.role]} className={buttonClass("primary")}>{t("common.goToMyArea")}</Link>}
          />
        </div>
      </div>
    );
  }
  if (requiredRoleFor(pathname) && requiredRoleFor(pathname) !== role) return null;

  const items = navByRole[role];
  const hrefs = items.map((i) => i.href);
  const wide = role !== "PARENT";

  return (
    <div className={cn("min-h-dvh pb-28 md:pb-0", role === "PARENT" ? "bg-dots" : "bg-dots-cool")}>
      <a href="#main" className="sr-only focus:not-sr-only focus:absolute focus:left-3 focus:top-3 focus:z-50 focus:rounded-full focus:bg-white focus:px-4 focus:py-2">
        {t("common.skipToContent")}
      </a>
      <header className="sticky top-0 z-40 border-b border-ink/5 bg-canvas/85 backdrop-blur-md">
        <div className={cn("mx-auto flex h-16 items-center justify-between gap-3 px-4 sm:px-6", wide ? "max-w-7xl" : "max-w-6xl")}>
          <Logo href={roleHome[role]} />
          <nav aria-label={t("common.mainNav")} className="hidden items-center gap-1 rounded-full bg-white p-1 shadow-clay-sm md:flex">
            {items.map((it) => {
              const Icon = icons[it.icon as keyof typeof icons];
              const active = isActive(pathname, it.href, hrefs);
              return (
                <Link
                  key={it.href}
                  href={it.href}
                  aria-current={active ? "page" : undefined}
                  className={cn(
                    "inline-flex h-10 items-center gap-2 rounded-full px-3.5 text-sm font-semibold transition-colors",
                    active ? "bg-pill text-white" : "text-ink-soft hover:text-ink",
                  )}
                >
                  <Icon className="h-[18px] w-[18px]" aria-hidden />
                  {t(`nav.${role.toLowerCase()}.${it.key}`)}
                </Link>
              );
            })}
          </nav>
          <div className="flex items-center gap-2">
            <LangSwitcher />
            <NotificationBell />
            <span className="hidden text-sm font-semibold lg:inline" title={user?.full_name}>{user?.full_name}</span>
            <Button
              variant="secondary"
              size="icon"
              aria-label={t("common.logout")}
              onClick={async () => { await logout(); router.replace("/login"); }}
            >
              <LogOut className="h-4 w-4" aria-hidden />
            </Button>
          </div>
        </div>
      </header>

      <main id="main" className={cn("mx-auto px-4 py-8 sm:px-6", wide ? "max-w-7xl" : "max-w-6xl")}>{children}</main>

      {role === "PARENT" && pathname !== "/parent/assistant" && (
        <Link
          href="/parent/assistant"
          aria-label={t("nav.parent.assistant")}
          className="group fixed bottom-24 right-4 z-50 grid h-14 w-14 place-items-center rounded-full border-2 border-ink bg-sun text-ink shadow-pop transition-all hover:-translate-y-1 hover:shadow-pop-lg active:translate-x-[2px] active:translate-y-[2px] active:shadow-none md:bottom-6 md:right-6"
        >
          <Sparkles className="h-6 w-6 transition-transform duration-300 group-hover:rotate-45" aria-hidden />
        </Link>
      )}

      <nav aria-label={t("common.mainNav")} className="fixed inset-x-3 bottom-3 z-40 rounded-[28px] border-2 border-ink bg-white/95 px-1 py-1.5 shadow-pop backdrop-blur md:hidden">
        <ul className="flex items-center justify-around">
          {items.slice(0, 5).map((it) => {
            const Icon = icons[it.icon as keyof typeof icons];
            const active = isActive(pathname, it.href, hrefs);
            return (
              <li key={it.href}>
                <Link href={it.href} aria-current={active ? "page" : undefined} className="flex min-w-14 flex-col items-center gap-0.5 rounded-2xl px-1.5 py-1">
                  <span className={cn("grid h-9 w-12 place-items-center rounded-full transition-colors", active ? "bg-pill text-white" : "text-ink-soft")}>
                    <Icon className="h-5 w-5" aria-hidden />
                  </span>
                  <span className={cn("text-[10px] font-semibold", active ? "text-ink" : "text-muted")}>{t(`nav.${role.toLowerCase()}.${it.key}`)}</span>
                </Link>
              </li>
            );
          })}
        </ul>
      </nav>
    </div>
  );
}
