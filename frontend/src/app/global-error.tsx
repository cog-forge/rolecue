"use client";
import { useSyncExternalStore } from "react";
import { defaultLocale } from "@/lib/i18n/config";
import { readBrowserLocale } from "@/lib/i18n/browser-locale";
import en from "@/lib/i18n/messages/en/common.json";
import vi from "@/lib/i18n/messages/vi/common.json";

const copy = { en: en.globalError, vi: vi.globalError };
const subscribe = () => () => {};

export default function GlobalError({
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  // This boundary replaces the root layout, so it resolves the locale itself.
  const locale = useSyncExternalStore(
    subscribe,
    readBrowserLocale,
    () => defaultLocale,
  );
  const text = copy[locale];
  return (
    <html lang={locale}>
      <body>
        <main style={{ padding: "3rem", fontFamily: "sans-serif" }}>
          <h1>{text.title}</h1>
          <p>{text.description}</p>
          <button onClick={reset}>{text.retry}</button>
        </main>
      </body>
    </html>
  );
}
