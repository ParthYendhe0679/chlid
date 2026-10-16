"use client";

import * as React from "react";
import { X } from "lucide-react";
import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils/cn";

/** Adaptive modal: bottom sheet on phones, centred modal on desktop. Uses native <dialog> for focus trap and Esc. */
export function Sheet({
  open, onClose, title, children, footer, wide,
}: { open: boolean; onClose: () => void; title: string; children: React.ReactNode; footer?: React.ReactNode; wide?: boolean }) {
  const ref = React.useRef<HTMLDialogElement>(null);
  const t = useTranslations("common");

  React.useEffect(() => {
    const d = ref.current;
    if (!d) return;
    if (open && !d.open) d.showModal();
    if (!open && d.open) d.close();
  }, [open]);

  return (
    <dialog
      ref={ref}
      aria-label={title}
      onClose={onClose}
      onClick={(e) => { if (e.target === ref.current) onClose(); }}
      className={cn(
        "m-0 w-full max-w-none bg-transparent p-0 backdrop:bg-ink/40 backdrop:backdrop-blur-[2px]",
        "fixed inset-x-0 bottom-0 top-auto sm:inset-0 sm:m-auto sm:h-fit",
        wide ? "sm:max-w-3xl" : "sm:max-w-lg",
      )}
    >
      <div className="flex max-h-[92dvh] flex-col rounded-t-[32px] border-2 border-ink bg-white shadow-float sm:rounded-[32px]">
        <div className="mx-auto mt-2.5 h-1.5 w-12 rounded-full bg-ink/15 sm:hidden" aria-hidden />
        <div className="flex items-start justify-between gap-3 px-6 pb-2 pt-4">
          <h2 className="text-xl font-bold">{title}</h2>
          <button
            type="button"
            onClick={onClose}
            aria-label={t("close")}
            className="grid h-10 w-10 shrink-0 place-items-center rounded-full bg-white shadow-clay-sm transition hover:rotate-90"
          >
            <X className="h-4 w-4" aria-hidden />
          </button>
        </div>
        <div className="overflow-y-auto px-6 pb-4 text-sm">{children}</div>
        {footer && <div className="flex flex-wrap justify-end gap-2 border-t border-line px-6 py-4">{footer}</div>}
      </div>
    </dialog>
  );
}
