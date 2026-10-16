"use client";

import * as React from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Bell, CheckCheck } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { cn } from "@/lib/utils/cn";
import { formatDateTime } from "@/lib/utils/format";
import { notificationsApi } from "@/lib/api/notifications.api";

/** In-app notifications (stage changes, deadlines, messages). Refreshes every minute and on focus. */
export function NotificationBell() {
  const t = useTranslations("notifications");
  const locale = useLocale();
  const qc = useQueryClient();
  const [open, setOpen] = React.useState(false);
  const ref = React.useRef<HTMLDivElement>(null);
  const q = useQuery({
    queryKey: ["notifications"],
    queryFn: ({ signal }) => notificationsApi.list(signal),
    refetchInterval: 60_000,
    refetchOnWindowFocus: true,
  });
  const readOne = useMutation({ mutationFn: notificationsApi.read, onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }) });
  const readAll = useMutation({ mutationFn: notificationsApi.readAll, onSuccess: () => qc.invalidateQueries({ queryKey: ["notifications"] }) });

  React.useEffect(() => {
    if (!open) return;
    const onDoc = (e: MouseEvent) => { if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false); };
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") setOpen(false); };
    document.addEventListener("mousedown", onDoc);
    document.addEventListener("keydown", onKey);
    return () => { document.removeEventListener("mousedown", onDoc); document.removeEventListener("keydown", onKey); };
  }, [open]);

  const unread = q.data?.unread ?? 0;
  const items = q.data?.items ?? [];

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        aria-label={unread ? t("labelUnread", { count: unread }) : t("label")}
        className="relative grid h-10 w-10 place-items-center rounded-full bg-white shadow-clay-sm hover:-translate-y-0.5"
      >
        <Bell className="h-[18px] w-[18px]" aria-hidden />
        {unread > 0 && (
          <span className="absolute -right-1 -top-1 grid h-5 min-w-5 place-items-center rounded-full border-2 border-white bg-coral px-1 text-[10px] font-bold text-white">
            {unread > 9 ? "9+" : unread}
          </span>
        )}
      </button>
      {open && (
        <div className="absolute right-0 z-50 mt-2 w-[min(22rem,calc(100vw-2rem))] overflow-hidden rounded-3xl border-2 border-ink bg-white shadow-pop">
          <div className="flex items-center justify-between border-b border-line px-4 py-3">
            <p className="text-sm font-bold">{t("title")}</p>
            {unread > 0 && (
              <button type="button" className="inline-flex items-center gap-1 text-xs font-semibold text-purple-deep" onClick={() => readAll.mutate()}>
                <CheckCheck className="h-3.5 w-3.5" aria-hidden /> {t("markAll")}
              </button>
            )}
          </div>
          <ul className="max-h-96 overflow-y-auto">
            {q.isError && <li className="px-4 py-6 text-center text-sm text-muted">{t("error")}</li>}
            {q.isSuccess && items.length === 0 && <li className="px-4 py-6 text-center text-sm text-muted">{t("empty")}</li>}
            {items.map((n) => {
              const body = (
                <>
                  <p className={cn("text-sm", n.read_at ? "font-medium text-ink-soft" : "font-bold")}>{n.title}</p>
                  <p className="mt-0.5 line-clamp-2 text-xs text-muted">{n.body}</p>
                  <p className="tabular mt-1 text-[10px] text-faint">{formatDateTime(n.created_at, locale)}</p>
                </>
              );
              const cls = cn("block border-b border-line px-4 py-3 text-left last:border-0 hover:bg-canvas", !n.read_at && "bg-lavender-soft/60");
              const onClick = () => { if (!n.read_at) readOne.mutate(n.id); setOpen(false); };
              return (
                <li key={n.id}>
                  {n.link ? <Link href={n.link} className={cls} onClick={onClick}>{body}</Link> : <button type="button" className={cn(cls, "w-full")} onClick={onClick}>{body}</button>}
                </li>
              );
            })}
          </ul>
        </div>
      )}
    </div>
  );
}
