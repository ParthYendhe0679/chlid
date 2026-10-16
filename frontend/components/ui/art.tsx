import * as React from "react";
import { cn } from "@/lib/utils/cn";

export function ArtTile({ icon: Icon, tone = "lavender", size = 48 }: { icon: React.ComponentType<{ className?: string; style?: React.CSSProperties }>; tone?: "lavender" | "sun" | "mint" | "sky" | "coral" | "pink"; size?: number }) {
  const bg = { lavender: "bg-lavender text-purple-deep", sun: "bg-sun text-sun-deep", mint: "bg-mint text-mint-deep", sky: "bg-sky text-sky-deep", coral: "bg-coral text-coral-deep", pink: "bg-pink text-pink-deep" }[tone];
  return (
    <span className={cn("relative inline-grid shrink-0 place-items-center overflow-hidden rounded-[30%] shadow-clay-sm", bg)} style={{ width: size, height: size }} aria-hidden>
      <span className="absolute inset-x-1 top-0.5 h-1/2 rounded-full bg-white/45 blur-[2px]" />
      <Icon className="relative drop-shadow-[0_2px_0_rgba(255,255,255,0.6)]" style={{ width: size * 0.52, height: size * 0.52 }} />
    </span>
  );
}

