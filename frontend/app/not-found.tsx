import Link from "next/link";
import { buttonClass } from "@/components/ui/button";

export default function NotFound() {
  return (
    <div className="bg-dots grid min-h-dvh place-items-center px-4">
      <div className="max-w-md rounded-[28px] border-2 border-ink bg-white p-8 text-center shadow-pop-lg">
        <p className="font-display text-5xl font-extrabold">404</p>
        <p className="mt-2 text-sm text-ink-soft">This page does not exist.</p>
        <Link href="/" className={`${buttonClass("primary")} mt-5`}>Home</Link>
      </div>
    </div>
  );
}
