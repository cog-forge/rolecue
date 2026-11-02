import { createTranslator } from "next-intl";
import type { AppLocale } from "@/lib/i18n/config";
import { messagesFor } from "@tests/helpers/intl";

// The translators a billing component gets from useTranslations(), for the plain functions in lib/.
export function compareFor(locale: AppLocale = "en") {
  return createTranslator({
    locale,
    messages: messagesFor(locale),
    namespace: "admin.billing.compare",
  });
}

export function datesFor(locale: AppLocale = "en") {
  return createTranslator({
    locale,
    messages: messagesFor(locale),
    namespace: "admin.billing.dates",
  });
}

export function timeWindowFor(locale: AppLocale = "en") {
  return createTranslator({
    locale,
    messages: messagesFor(locale),
    namespace: "admin.billing.timeWindow",
  });
}
