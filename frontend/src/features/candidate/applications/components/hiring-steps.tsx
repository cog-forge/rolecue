import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";

const steps = ["review", "interview", "decision"] as const;

// What happens after applying. With current set (0-based), that step is marked as the one under way.
export function HiringSteps({
  current,
  className,
}: {
  current?: number;
  className?: string;
}) {
  const t = useTranslations("candidate.applications.hiringSteps");
  return (
    <ol className={cn("space-y-3", className)}>
      {steps.map((step, index) => {
        const active = index === current;
        return (
          <li
            key={step}
            aria-current={active ? "step" : undefined}
            className="flex gap-3"
          >
            <span
              aria-hidden
              className={cn(
                "flex size-6 shrink-0 items-center justify-center rounded-full border text-xs font-semibold tabular-nums",
                active && "border-foreground bg-foreground text-background",
              )}
            >
              {index + 1}
            </span>
            <div className="min-w-0 text-sm">
              <p className="font-medium">
                {t(`${step}.title`)}
                {active && <span className="sr-only">{t("current")}</span>}
              </p>
              <p className="text-muted-foreground">{t(`${step}.detail`)}</p>
            </div>
          </li>
        );
      })}
    </ol>
  );
}
