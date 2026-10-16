const LOCALE_TAG = { en: "en-IN", hi: "hi-IN", mr: "mr-IN" } as const;
type L = keyof typeof LOCALE_TAG;

export function formatDate(iso: string | null | undefined, locale: string = "en"): string {
  if (!iso) return "-";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "-";
  return new Intl.DateTimeFormat(LOCALE_TAG[(locale as L)] ?? "en-IN", { day: "numeric", month: "short", year: "numeric" }).format(d);
}

export function formatDateTime(iso: string | null | undefined, locale: string = "en"): string {
  if (!iso) return "-";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "-";
  return new Intl.DateTimeFormat(LOCALE_TAG[(locale as L)] ?? "en-IN", { day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" }).format(d);
}

export function formatINR(n: number, locale: string = "en"): string {
  return new Intl.NumberFormat(LOCALE_TAG[(locale as L)] ?? "en-IN", { style: "currency", currency: "INR", maximumFractionDigits: 0 }).format(n);
}

export function formatNumber(n: number, locale: string = "en"): string {
  return new Intl.NumberFormat(LOCALE_TAG[(locale as L)] ?? "en-IN").format(n);
}

/** Whole milliseconds remaining until an ISO instant, never negative. */
export function msUntil(iso: string, now = Date.now()): number {
  return Math.max(0, new Date(iso).getTime() - now);
}

export function splitDuration(ms: number) {
  const total = Math.floor(ms / 1000);
  return { h: Math.floor(total / 3600), m: Math.floor((total % 3600) / 60), s: total % 60 };
}
