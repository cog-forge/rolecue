import type { ReactNode } from "react";
import { CircleCheck, CircleDashed, MonitorSmartphone } from "lucide-react";
import { useTranslations } from "next-intl";

type Item = {
  label: string;
  value: string;
  // Protected states carry the brand hue; open items stay in ink with a dashed mark,
  // and a count with no good or bad state takes a neutral mark.
  ok: boolean | "neutral";
  detail: string;
  href: string;
};

// The screen's first answer: what protects this account and who is signed in.
export function SecuritySummary({
  emailVerified,
  hasPassword,
  twoFactorEnabled,
  sessionCount,
}: {
  emailVerified: boolean;
  hasPassword?: boolean;
  twoFactorEnabled: boolean;
  sessionCount?: number;
}) {
  const t = useTranslations("auth.security");
  const items: Item[] = [
    {
      label: t("summary.email"),
      value: emailVerified ? t("status.verified") : t("status.notVerified"),
      ok: emailVerified,
      detail: emailVerified
        ? t("summary.emailVerifiedDetail")
        : t("summary.emailUnverifiedDetail"),
      href: "#sign-in-methods",
    },
    {
      label: t("summary.password"),
      value:
        hasPassword === undefined
          ? "—"
          : hasPassword
            ? t("status.set")
            : t("status.notSet"),
      ok: Boolean(hasPassword),
      detail:
        hasPassword === false
          ? t("summary.passwordSocialOnly")
          : t("summary.passwordEmail"),
      href: "#password-settings",
    },
    {
      label: t("summary.twoFactor"),
      value: twoFactorEnabled ? t("status.on") : t("status.off"),
      ok: twoFactorEnabled,
      detail: twoFactorEnabled
        ? t("summary.twoFactorOnDetail")
        : t("summary.twoFactorOffDetail"),
      href: "#two-factor-settings",
    },
    {
      label: t("summary.signedIn"),
      value:
        sessionCount === undefined
          ? "—"
          : t("summary.sessions", {
              count: sessionCount,
              countText: String(sessionCount),
            }),
      ok: "neutral",
      detail:
        sessionCount === undefined || sessionCount < 2
          ? t("summary.thisBrowserOnly")
          : t("summary.thisBrowserAndMore", {
              countText: String(sessionCount - 1),
            }),
      href: "#active-sessions",
    },
  ];
  return (
    <nav
      aria-label={t("summary.label")}
      className="grid grid-cols-2 overflow-hidden rounded-2xl border lg:grid-cols-4"
    >
      {items.map((item, index) => (
        <SummaryCell key={item.label} index={index} item={item} />
      ))}
    </nav>
  );
}

function SummaryCell({ item, index }: { item: Item; index: number }) {
  const mark: ReactNode =
    item.ok === "neutral" ? (
      <MonitorSmartphone className="size-4 text-muted-foreground" aria-hidden />
    ) : item.ok ? (
      <CircleCheck className="size-4 text-rolecue-brand" aria-hidden />
    ) : (
      <CircleDashed className="size-4 text-muted-foreground" aria-hidden />
    );
  return (
    <a
      href={item.href}
      className={[
        "group flex min-h-24 flex-col justify-between gap-3 p-4 outline-none transition-colors duration-(--duration-quick) hover:bg-muted/60 focus-visible:bg-muted/60 focus-visible:ring-3 focus-visible:ring-ring/40 focus-visible:ring-inset sm:p-5",
        index % 2 === 1 ? "border-l" : "",
        index >= 2 ? "border-t lg:border-t-0" : "",
        index === 2 ? "lg:border-l" : "",
      ].join(" ")}
    >
      <span className="text-sm text-muted-foreground">{item.label}</span>
      <span className="space-y-1">
        <span className="flex items-center gap-2 text-lg font-medium tabular-nums">
          {mark}
          {item.value}
        </span>
        <span className="block text-xs text-muted-foreground group-hover:text-foreground">
          {item.detail}
        </span>
      </span>
    </a>
  );
}
