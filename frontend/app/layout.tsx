import type { Metadata, Viewport } from "next";
import { Unbounded, Poppins, Lobster } from "next/font/google";
import { NextIntlClientProvider } from "next-intl";
import { getLocale, getMessages } from "next-intl/server";
import { Providers } from "@/components/layout/providers";
import { DemoBanner } from "@/components/layout/brand";
import { DEMO_MODE } from "@/lib/api/client";
import "./globals.css";

const unbounded = Unbounded({ variable: "--font-unbounded", subsets: ["latin"], weight: ["500", "600", "700", "800"], display: "swap" });
const lobster = Lobster({ variable: "--font-lobster", subsets: ["latin"], weight: "400", display: "swap" });
const poppins = Poppins({ variable: "--font-poppins", subsets: ["latin", "devanagari"], weight: ["400", "500", "600", "700"], display: "swap" });

export const metadata: Metadata = {
  title: { default: "Child Connect", template: "%s | Child Connect" },
  description: "A secure helper for the legal child adoption process in India. AI helps with paperwork and deadlines; a human always decides.",
};
export const viewport: Viewport = { themeColor: "#f5f1e8", width: "device-width", initialScale: 1 };

export default async function RootLayout({ children }: { children: React.ReactNode }) {
  const locale = await getLocale();
  const messages = await getMessages();
  return (
    <html lang={locale} className={`${unbounded.variable} ${poppins.variable} ${lobster.variable}`}>
      <body className="min-h-dvh">
        <NextIntlClientProvider locale={locale} messages={messages}>
          {DEMO_MODE && <DemoBanner />}
          <Providers>{children}</Providers>
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
