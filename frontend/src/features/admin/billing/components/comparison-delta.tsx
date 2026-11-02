"use client";

import { useLocale, useTranslations } from "next-intl";
import { ArrowDownRight, ArrowUpRight } from "lucide-react";
import { cn } from "@/lib/utils";
import { compareValues, type ComparisonKind } from "../lib/compare";

// The change against the comparison window: colour says good or bad and the arrow and sign carry it
// without colour. Rates show their point change only, since a percentage of a percentage misleads.
export function ComparisonDelta({
  current,
  previous,
  kind,
  showAbsolute = true,
  className,
}: {
  current: number;
  previous: number;
  kind: ComparisonKind;
  showAbsolute?: boolean;
  className?: string;
}) {
  const locale = useLocale();
  const t = useTranslations("admin.billing.compare");
  const comparison = compareValues(current, previous, kind, locale, t);
  if (comparison.state === "new") {
    return (
      <span className={cn("text-muted-foreground", className)}>{t("new")}</span>
    );
  }
  if (comparison.state === "same") {
    return (
      <span className={cn("text-muted-foreground", className)}>
        {t("noChange")}
      </span>
    );
  }
  const Icon = comparison.direction === "up" ? ArrowUpRight : ArrowDownRight;
  const isRate = comparison.kind === "rate";
  return (
    <span
      className={cn("inline-flex items-center gap-1 tabular-nums", className)}
    >
      <span
        className={cn(
          "inline-flex items-center gap-0.5 font-medium",
          comparison.tone === "positive"
            ? "text-[color:var(--rolecue-cue-green)]"
            : "text-destructive",
        )}
      >
        <Icon className="size-3.5" aria-hidden />
        {isRate ? comparison.absolute : comparison.percent}
      </span>
      {showAbsolute && !isRate && (
        <span className="text-muted-foreground">({comparison.absolute})</span>
      )}
    </span>
  );
}
