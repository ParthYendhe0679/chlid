"use client";

import * as React from "react";
import { Check } from "lucide-react";
import { cn } from "@/lib/utils/cn";

const fieldBase =
  "w-full rounded-2xl border-2 border-transparent bg-white shadow-clay-sm outline-none transition-colors placeholder:text-faint focus:border-purple aria-[invalid=true]:border-danger disabled:opacity-60";

export const Input = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(function Input(
  { className, ...props },
  ref,
) {
  return <input ref={ref} className={cn(fieldBase, "h-12 px-4 text-sm font-medium", className)} {...props} />;
});

export const Textarea = React.forwardRef<HTMLTextAreaElement, React.TextareaHTMLAttributes<HTMLTextAreaElement>>(function Textarea(
  { className, ...props },
  ref,
) {
  return <textarea ref={ref} className={cn(fieldBase, "min-h-28 resize-y px-4 py-3 text-sm font-medium", className)} {...props} />;
});

const chevron =
  "bg-[url('data:image/svg+xml;utf8,<svg xmlns=%22http://www.w3.org/2000/svg%22 width=%2212%22 height=%2212%22 viewBox=%220 0 256 256%22><path fill=%22%231c1b2b%22 d=%22M213.66 101.66l-80 80a8 8 0 0 1-11.32 0l-80-80a8 8 0 0 1 11.32-11.32L128 164.69l74.34-74.35a8 8 0 0 1 11.32 11.32Z%22/></svg>')] bg-[length:12px] bg-[right_14px_center] bg-no-repeat pr-9";

export const Select = React.forwardRef<HTMLSelectElement, React.SelectHTMLAttributes<HTMLSelectElement>>(function Select(
  { className, children, ...props },
  ref,
) {
  return (
    <select ref={ref} className={cn(fieldBase, "h-11 appearance-none px-4 text-sm font-medium", chevron, className)} {...props}>
      {children}
    </select>
  );
});

export function Field({
  label, hint, error, htmlFor, children, className,
}: { label: string; hint?: string; error?: string; htmlFor: string; children: React.ReactNode; className?: string }) {
  return (
    <div className={cn("space-y-1.5", className)}>
      <label htmlFor={htmlFor} className="block text-[13px] font-bold text-ink-soft">{label}</label>
      {children}
      {hint && !error && <p id={`${htmlFor}-hint`} className="text-xs font-medium text-muted">{hint}</p>}
      {error && <p id={`${htmlFor}-error`} role="alert" className="text-xs font-semibold text-danger">{error}</p>}
    </div>
  );
}

const toneBg = { purple: "bg-lavender", sun: "bg-sun", mint: "bg-mint", sky: "bg-sky", coral: "bg-coral", pink: "bg-pink" } as const;

export function Chip({
  selected, onClick, children, tone = "sun", size = "md", disabled,
}: { selected?: boolean; onClick?: () => void; children: React.ReactNode; tone?: keyof typeof toneBg; size?: "sm" | "md"; disabled?: boolean }) {
  return (
    <button
      type="button"
      role="checkbox"
      aria-checked={!!selected}
      disabled={disabled}
      onClick={onClick}
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full font-semibold transition-all active:scale-95 disabled:opacity-50",
        size === "sm" ? "h-8 px-3 text-xs" : "h-10 px-4 text-sm",
        selected ? cn(toneBg[tone], "text-ink ring-2 ring-ink shadow-pop") : "bg-white text-ink-soft shadow-clay-sm hover:-translate-y-0.5",
      )}
    >
      {selected && <Check className="h-3.5 w-3.5" aria-hidden />}
      {children}
    </button>
  );
}

export function Switch({ checked, onChange, label, id }: { checked: boolean; onChange: (v: boolean) => void; label: string; id: string }) {
  return (
    <div className="flex items-center justify-between gap-3">
      <label htmlFor={id} className="text-sm font-semibold text-ink-soft">{label}</label>
      <button
        id={id}
        type="button"
        role="switch"
        aria-checked={checked}
        onClick={() => onChange(!checked)}
        className={cn("relative h-7 w-12 shrink-0 rounded-full transition-colors", checked ? "bg-purple" : "bg-ink/15")}
      >
        <span className={cn("absolute top-1 h-5 w-5 rounded-full bg-white shadow-md transition-transform", checked ? "translate-x-6" : "translate-x-1")} />
      </button>
    </div>
  );
}
