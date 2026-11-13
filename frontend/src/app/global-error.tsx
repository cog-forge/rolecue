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
      <body
        style={{
          margin: 0,
          minHeight: "100vh",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          fontFamily:
            '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif',
          backgroundColor: "#fafafa",
          color: "#262626",
          padding: "1.5rem",
        }}
      >
        <main
          role="alert"
          style={{
            maxWidth: "32rem",
            width: "100%",
            backgroundColor: "#ffffff",
            borderRadius: "1.25rem",
            border: "1px solid #e5e5e5",
            padding: "2.5rem 2rem",
            boxShadow: "0 10px 25px -5px rgba(0, 0, 0, 0.05)",
            textAlign: "center",
          }}
        >
          <div
            style={{
              width: "3.5rem",
              height: "3.5rem",
              borderRadius: "1rem",
              backgroundColor: "#fee2e2",
              color: "#dc2626",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              margin: "0 auto 1.25rem auto",
            }}
          >
            <svg
              width="28"
              height="28"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z" />
              <line x1="12" y1="9" x2="12" y2="13" />
              <line x1="12" y1="17" x2="12.01" y2="17" />
            </svg>
          </div>

          <h1
            style={{
              fontSize: "1.5rem",
              fontWeight: 600,
              margin: "0 0 0.5rem 0",
              letterSpacing: "-0.025em",
            }}
          >
            {text.title}
          </h1>

          <p
            style={{
              fontSize: "0.9375rem",
              lineHeight: 1.5,
              color: "#595959",
              margin: "0 0 1.5rem 0",
            }}
          >
            {text.description}
          </p>

          <button
            type="button"
            onClick={reset}
            style={{
              display: "inline-flex",
              alignItems: "center",
              justifyContent: "center",
              height: "2.5rem",
              padding: "0 1.5rem",
              fontSize: "0.875rem",
              fontWeight: 500,
              color: "#ffffff",
              backgroundColor: "#185cc0",
              borderRadius: "0.75rem",
              border: "none",
              cursor: "pointer",
              transition: "background-color 0.2s ease",
            }}
          >
            {text.retry}
          </button>
        </main>
      </body>
    </html>
  );
}
