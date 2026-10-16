"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { Globe, HeartHandshake } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { LANG_LABEL, setLocaleCookie } from "@/lib/utils/locale";
import type { Lang } from "@/types";

export function Logo({ href = "/" }: { href?: string }) {
  return (
    <Link href={href} className="flex items-center gap-2 font-display text-lg font-extrabold tracking-tight">
      <span className="grid h-9 w-9 place-items-center rounded-[30%] border-2 border-ink bg-sun shadow-pop">
        <HeartHandshake className="h-5 w-5" aria-hidden />
      </span>
      Child Connect
    </Link>
  );
}

export function LangSwitcher() {
  const locale = useLocale() as Lang;
  const router = useRouter();
  const t = useTranslations("common");
  return (
    <label className="relative inline-flex items-center">
      <span className="sr-only">{t("language")}</span>
      <Globe className="pointer-events-none absolute left-3 h-4 w-4" aria-hidden />
      <select
        value={locale}
        onChange={(e) => { setLocaleCookie(e.target.value as Lang); router.refresh(); }}
        className="h-10 cursor-pointer appearance-none rounded-full bg-white pl-9 pr-4 text-sm font-semibold shadow-clay-sm outline-none"
      >
        {(Object.keys(LANG_LABEL) as Lang[]).map((l) => (
          <option key={l} value={l}>{LANG_LABEL[l]}</option>
        ))}
      </select>
    </label>
  );
}

export function DemoBanner() {
  return (
    <div role="note" className="bg-sun px-4 py-1.5 text-center text-xs font-bold text-ink">
      DEMO MODE: synthetic data only. No real backend, no real AI analysis, no real legal information.
    </div>
  );
}
