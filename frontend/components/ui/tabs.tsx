"use client";

import * as React from "react";
import { cn } from "@/lib/utils/cn";

export interface TabDef { key: string; label: string; badge?: React.ReactNode }

/** Accessible segmented tabs. Panels are rendered by the caller via the `active` key. */
export function Tabs({ tabs, active, onChange, label }: { tabs: TabDef[]; active: string; onChange: (k: string) => void; label: string }) {
  const refs = React.useRef<Record<string, HTMLButtonElement | null>>({});
  const onKey = (e: React.KeyboardEvent, i: number) => {
    let n = i;
    if (e.key === "ArrowRight") n = (i + 1) % tabs.length;
    else if (e.key === "ArrowLeft") n = (i - 1 + tabs.length) % tabs.length;
    else return;
    e.preventDefault();
    onChange(tabs[n].key);
    refs.current[tabs[n].key]?.focus();
  };
  return (
    <div role="tablist" aria-label={label} className="flex max-w-full gap-1 overflow-x-auto rounded-full bg-white p-1 shadow-clay-sm">
      {tabs.map((t, i) => (
        <button
          key={t.key}
          ref={(el) => { refs.current[t.key] = el; }}
          role="tab"
          id={`tab-${t.key}`}
          aria-selected={active === t.key}
          aria-controls={`panel-${t.key}`}
          tabIndex={active === t.key ? 0 : -1}
          onClick={() => onChange(t.key)}
          onKeyDown={(e) => onKey(e, i)}
          className={cn(
            "inline-flex h-10 shrink-0 items-center gap-2 rounded-full px-4 text-sm font-semibold transition-colors",
            active === t.key ? "bg-pill text-white" : "text-ink-soft hover:text-ink",
          )}
        >
          {t.label}
          {t.badge}
        </button>
      ))}
    </div>
  );
}

export function TabPanel({ id, active, children }: { id: string; active: string; children: React.ReactNode }) {
  if (id !== active) return null;
  return <div role="tabpanel" id={`panel-${id}`} aria-labelledby={`tab-${id}`} tabIndex={0} className="mt-5 outline-none">{children}</div>;
}
