import { defaultLocale, type AppLocale } from "./config";
import { findMissingKeys, mergeMessages } from "./fallback";
import type en from "./messages/en";

export type Messages = typeof en;

const loaders: Record<AppLocale, () => Promise<{ default: unknown }>> = {
  en: () => import("./messages/en"),
  vi: () => import("./messages/vi"),
};

// Messages for a locale with English filled in for any missing key. In development the
// missing keys are logged so translators notice them; the page never fails because of one.
export async function loadMessages(locale: AppLocale): Promise<Messages> {
  const base = (await loaders[defaultLocale]()).default as Messages;
  if (locale === defaultLocale) return base;
  const translation = (await loaders[locale]()).default as Messages;
  if (process.env.NODE_ENV === "development") {
    const missing = findMissingKeys(base, translation);
    if (missing.length > 0)
      console.warn(
        `[i18n] ${missing.length} message key(s) missing in "${locale}", showing English: ${missing.join(", ")}`,
      );
  }
  return mergeMessages(base, translation);
}
