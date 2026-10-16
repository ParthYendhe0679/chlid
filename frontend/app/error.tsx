"use client";

import { Button } from "@/components/ui/button";

export default function Error({ reset }: { error: Error; reset: () => void }) {
  return (
    <div className="bg-dots grid min-h-dvh place-items-center px-4">
      <div role="alert" className="max-w-md rounded-[28px] border-2 border-dashed border-danger/30 bg-white p-8 text-center">
        <p className="font-display text-xl font-bold">Something went wrong</p>
        <p className="mt-2 text-sm text-ink-soft">Please try again. Your data has not been changed.</p>
        <Button className="mt-5" onClick={reset}>Try again</Button>
      </div>
    </div>
  );
}
