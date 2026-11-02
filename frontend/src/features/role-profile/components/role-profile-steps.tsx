"use client";

import { Check } from "lucide-react";
import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";

const steps = ["input", "review", "confirmed"] as const;

export function RoleProfileSteps({
  current,
}: {
  current: (typeof steps)[number];
}) {
  const t = useTranslations("candidate.roleProfiles.steps");
  const active = steps.indexOf(current);
  return (
    <ol
      aria-label={t("ariaLabel")}
      className="flex flex-wrap items-center gap-x-5 gap-y-3 text-xs sm:gap-x-8"
    >
      {steps.map((step, index) => (
        <li
          key={step}
          aria-current={index === active ? "step" : undefined}
          className={cn(
            "flex items-center gap-2",
            index === active
              ? "font-medium text-foreground"
              : "text-muted-foreground",
          )}
        >
          <span
            aria-hidden="true"
            className={cn(
              "flex size-6 shrink-0 items-center justify-center rounded-full",
              index < active && "bg-workspace-accent/10 text-workspace-accent",
              index === active && "bg-foreground text-background",
              index > active && "border",
            )}
          >
            {index < active ? <Check className="size-3.5" /> : index + 1}
          </span>
          {t(step)}
          <span className="sr-only">
            {index < active
              ? t("complete")
              : index === active
                ? t("current")
                : t("upcoming")}
          </span>
        </li>
      ))}
    </ol>
  );
}
