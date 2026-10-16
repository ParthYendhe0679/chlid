import * as React from "react";
import { Loader2 } from "lucide-react";
import { cn } from "@/lib/utils/cn";

const variants = {
  primary:
    "bg-pill text-white border-2 border-pill shadow-[3px_3px_0_#7b5cf0] hover:shadow-[4px_4px_0_#7b5cf0] hover:-translate-y-0.5 active:translate-x-[2px] active:translate-y-[2px] active:shadow-none",
  purple:
    "bg-purple text-white border-2 border-ink shadow-pop hover:-translate-y-0.5 active:translate-x-[2px] active:translate-y-[2px] active:shadow-none",
  secondary: "bg-white text-ink shadow-clay-sm hover:-translate-y-0.5 active:scale-[0.97]",
  outline:
    "bg-white text-ink border-2 border-ink shadow-pop hover:-translate-y-0.5 active:translate-x-[2px] active:translate-y-[2px] active:shadow-none",
  soft: "bg-lavender-soft text-purple-deep hover:bg-lavender/60 active:scale-[0.97]",
  danger: "bg-danger text-white hover:brightness-110 active:scale-[0.97]",
  success: "bg-success text-white hover:brightness-110 active:scale-[0.97]",
  ghost: "bg-transparent text-ink hover:bg-ink/5 active:scale-[0.97]",
} as const;

const sizes = {
  sm: "h-9 px-3.5 text-[13px] gap-1.5",
  md: "h-11 px-5 text-sm gap-2",
  lg: "h-14 px-7 text-base gap-2.5",
  icon: "h-11 w-11 p-0",
  "icon-sm": "h-9 w-9 p-0",
} as const;

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: keyof typeof variants;
  size?: keyof typeof sizes;
  loading?: boolean;
}

export const buttonClass = (variant: keyof typeof variants = "primary", size: keyof typeof sizes = "md", className?: string) =>
  cn(
    "relative inline-flex select-none items-center justify-center whitespace-nowrap rounded-full font-semibold transition-all duration-200 disabled:pointer-events-none disabled:opacity-50 aria-disabled:pointer-events-none aria-disabled:opacity-50",
    variants[variant],
    sizes[size],
    className,
  );

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = "primary", size = "md", loading, disabled, className, children, type = "button", ...props },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={buttonClass(variant, size, className)}
      {...props}
    >
      {loading && <Loader2 className="h-4 w-4 animate-spin motion-reduce:animate-none" aria-hidden />}
      {children}
    </button>
  );
});
