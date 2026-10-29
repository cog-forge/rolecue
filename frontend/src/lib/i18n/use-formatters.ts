"use client";

import { useMemo } from "react";
import { useLocale, useTranslations } from "next-intl";
import { createFormatters } from "./formatters";

// Locale-aware formatting for Client Components; re-renders with the language switcher.
export function useFormatters() {
  const locale = useLocale();
  const t = useTranslations("common.format");
  return useMemo(() => createFormatters(locale, t), [locale, t]);
}
