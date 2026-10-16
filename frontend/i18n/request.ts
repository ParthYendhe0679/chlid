import { getRequestConfig } from "next-intl/server";
import { cookies } from "next/headers";
import en from "@/messages/en.json";

export const LOCALES = ["en", "hi", "mr"] as const;
export type Locale = (typeof LOCALES)[number];
export const LOCALE_COOKIE = "cc_locale";

type Dict = { [k: string]: string | Dict };

/** Fill any key missing in hi/mr from English so untranslated screens never show raw keys. */
function merge(base: Dict, over: Dict): Dict {
  const out: Dict = { ...base };
  for (const [k, v] of Object.entries(over)) {
    const b = out[k];
    out[k] = typeof v === "object" && typeof b === "object" ? merge(b, v) : v;
  }
  return out;
}

export default getRequestConfig(async () => {
  const store = await cookies();
  const raw = store.get(LOCALE_COOKIE)?.value;
  const locale: Locale = (LOCALES as readonly string[]).includes(raw ?? "") ? (raw as Locale) : "en";
  const messages = locale === "en" ? (en as Dict) : merge(en as Dict, (await import(`@/messages/${locale}.json`)).default as Dict);
  return { locale, messages };
});
