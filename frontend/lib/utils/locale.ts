import type { Lang } from "@/types";

export const LOCALE_COOKIE = "cc_locale";

/** Language preference only; contains no personal data. Persisted for one year. */
export function setLocaleCookie(lang: Lang) {
  if (typeof document === "undefined") return;
  document.cookie = `${LOCALE_COOKIE}=${lang}; path=/; max-age=31536000; SameSite=Lax`;
}

export const LANG_LABEL: Record<Lang, string> = { en: "English", hi: "हिन्दी", mr: "मराठी" };
export const SPEECH_LOCALE: Record<Lang, string> = { en: "en-IN", hi: "hi-IN", mr: "mr-IN" };
