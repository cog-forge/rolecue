// Single source of truth for the locales the UI supports. Adding a locale starts here.
export const locales = ["en", "vi"] as const;
export type AppLocale = (typeof locales)[number];

export const defaultLocale: AppLocale = "en";

// Cookie written by the language switcher and read on every request.
export const localeCookieName = "NEXT_LOCALE";
export const localeCookieMaxAge = 60 * 60 * 24 * 365;

// Each language is always shown in its own name, whatever the active locale is.
export const localeNames: Record<AppLocale, string> = {
  en: "English",
  vi: "Tiếng Việt",
};

export function isLocale(value: unknown): value is AppLocale {
  return (
    typeof value === "string" && locales.some((locale) => locale === value)
  );
}
