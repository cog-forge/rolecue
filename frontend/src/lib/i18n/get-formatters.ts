import { getLocale, getTranslations } from "next-intl/server";
import { createFormatters } from "./formatters";

// Locale-aware formatting for Server Components and route handlers.
export async function getFormatters() {
  const [locale, t] = await Promise.all([
    getLocale(),
    getTranslations("common.format"),
  ]);
  return createFormatters(locale, t);
}
