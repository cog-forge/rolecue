import type { ReactNode } from "react";
import { CircleCheck, CircleDashed, MonitorSmartphone } from "lucide-react";

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
  const items: Item[] = [
    {
      label: "Email",
      value: emailVerified ? "Verified" : "Not verified",
      ok: emailVerified,
      detail: emailVerified ? "Used for recovery" : "Check your inbox",
      href: "#sign-in-methods",
    },
    {
      label: "Password",
      value: hasPassword === undefined ? "—" : hasPassword ? "Set" : "Not set",
      ok: Boolean(hasPassword),
      detail: hasPassword === false ? "Social sign-in only" : "Email sign-in",
      href: "#password-settings",
    },
    {
      label: "Two-factor",
      value: twoFactorEnabled ? "On" : "Off",
      ok: twoFactorEnabled,
      detail: twoFactorEnabled ? "Authenticator app" : "Recommended",
      href: "#two-factor-settings",
    },
    {
      label: "Signed in",
      value:
        sessionCount === undefined
          ? "—"
          : `${sessionCount} ${sessionCount === 1 ? "session" : "sessions"}`,
      ok: "neutral",
      detail:
        sessionCount === undefined || sessionCount < 2
          ? "This browser only"
          : `This browser and ${sessionCount - 1} more`,
      href: "#active-sessions",
    },
  ];
  return (
    <nav
      aria-label="Security summary"
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
