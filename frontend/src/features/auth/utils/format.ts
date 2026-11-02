import type { useTranslations } from "next-intl";
import type { Formatters } from "@/lib/i18n/formatters";

// `auth.security` translator; plain functions receive it rather than reading the locale.
export type SecurityTranslator = ReturnType<
  typeof useTranslations<"auth.security">
>;

export function securityDate(
  value: Date | string | null | undefined,
  fmt: Formatters,
  t: SecurityTranslator,
) {
  if (!value) return t("unavailable");
  const text = fmt.date(value, "calendarDayTime");
  return text ? t("format.utc", { date: text }) : t("unavailable");
}

export function securityDay(
  value: Date | string | null | undefined,
  fmt: Formatters,
  t: SecurityTranslator,
) {
  if (!value) return t("unavailable");
  return fmt.date(value, "calendarDay") || t("unavailable");
}

export function isMobileAgent(agent?: string | null) {
  return Boolean(agent && /Android|iPhone|iPad|Mobile/.test(agent));
}

export function deviceLabel(
  agent: string | null | undefined,
  t: SecurityTranslator,
) {
  if (!agent) return t("device.unknownDevice");
  const browser = /Edg\//.test(agent)
    ? "Edge"
    : /Firefox\//.test(agent)
      ? "Firefox"
      : /(?:Chrome|CriOS)\//.test(agent)
        ? "Chrome"
        : /Safari\//.test(agent)
          ? "Safari"
          : t("device.unknownBrowser");
  const system = /Android/.test(agent)
    ? "Android"
    : /(?:iPhone|iPad)/.test(agent)
      ? "iOS"
      : /Windows/.test(agent)
        ? "Windows"
        : /Macintosh/.test(agent)
          ? "macOS"
          : /Linux/.test(agent)
            ? "Linux"
            : t("device.unknownSystem");
  return t("device.label", { browser, system });
}
