import * as React from "react";
import { cn } from "@/lib/utils/cn";

const variants = {
  clay: "bg-white shadow-clay rounded-[28px]",
  bento: "bg-white border border-line-cool shadow-soft rounded-2xl",
  sticker: "bg-white border-2 border-ink shadow-pop-lg rounded-[24px]",
  flat: "bg-white rounded-2xl border border-line",
  lavender: "rounded-[24px] bg-lavender-soft",
  sun: "rounded-[24px] bg-sun-soft",
  mint: "rounded-[24px] bg-mint-soft",
  sky: "rounded-[24px] bg-sky-soft",
  coral: "rounded-[24px] bg-coral-soft",
  pink: "rounded-[24px] bg-pink-soft",
} as const;

export interface CardProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: keyof typeof variants;
  pad?: boolean;
  interactive?: boolean;
}

export function Card({ variant = "clay", pad = true, interactive, className, ...props }: CardProps) {
  return (
    <div
      className={cn(
        variants[variant],
        pad && "p-5 sm:p-6",
        interactive && "cursor-pointer transition-all duration-200 hover:-translate-y-1 hover:shadow-clay-lg",
        className,
      )}
      {...props}
    />
  );
}

export function SectionTitle({ kicker, title, children }: { kicker?: string; title: string; children?: React.ReactNode }) {
  return (
    <div className="mb-5 flex flex-wrap items-end justify-between gap-3">
      <div>
        {kicker && (
          <span className="mb-2 inline-block -rotate-1 rounded-full border-2 border-ink bg-sun px-3 py-1 text-xs font-bold uppercase tracking-[0.14em] shadow-pop">
            {kicker}
          </span>
        )}
        <h2 className="text-2xl font-extrabold leading-[1.1] sm:text-3xl">{title}</h2>
      </div>
      {children}
    </div>
  );
}

export function PageHead({ title, subtitle, kicker, children }: { title: string; subtitle?: string; kicker?: string; children?: React.ReactNode }) {
  return (
    <div className="mb-6 flex flex-wrap items-start justify-between gap-4">
      <div className="max-w-2xl">
        {kicker && (
          <span className="mb-3 inline-block -rotate-1 rounded-full border-2 border-ink bg-sun px-3 py-1 text-xs font-bold uppercase tracking-[0.14em] shadow-pop">
            {kicker}
          </span>
        )}
        <h1 className="text-3xl font-extrabold leading-[1.08] sm:text-4xl">{title}</h1>
        {subtitle && <p className="mt-2 text-[15px] leading-relaxed text-ink-soft">{subtitle}</p>}
      </div>
      {children && <div className="flex flex-wrap items-center gap-2">{children}</div>}
    </div>
  );
}
