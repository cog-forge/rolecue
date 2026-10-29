import type { Formatters } from "@/lib/i18n/formatters";

export function securityDate(
  value: Date | string | null | undefined,
  fmt: Formatters,
) {
  if (!value) return "Unavailable";
  const text = fmt.date(value, "calendarDayTime");
  return text ? `${text} UTC` : "Unavailable";
}

export function securityDay(
  value: Date | string | null | undefined,
  fmt: Formatters,
) {
  if (!value) return "Unavailable";
  return fmt.date(value, "calendarDay") || "Unavailable";
}

export function isMobileAgent(agent?: string | null) {
  return Boolean(agent && /Android|iPhone|iPad|Mobile/.test(agent));
}

export function deviceLabel(agent?: string | null) {
  if (!agent) return "Unknown device";
  const browser = /Edg\//.test(agent)
    ? "Edge"
    : /Firefox\//.test(agent)
      ? "Firefox"
      : /(?:Chrome|CriOS)\//.test(agent)
        ? "Chrome"
        : /Safari\//.test(agent)
          ? "Safari"
          : "Unknown browser";
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
            : "unknown device";
  return `${browser} on ${system}`;
}
