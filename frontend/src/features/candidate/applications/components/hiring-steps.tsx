import { cn } from "@/lib/utils";

const steps = [
  {
    title: "The recruiter reviews your CV",
    detail: "No interview starts until your CV is approved.",
  },
  {
    title: "Complete the AI interview within 24 hours",
    detail:
      "If your CV is approved, the interview opens in My Applications. It is free for you.",
  },
  {
    title: "The recruiter makes the final decision",
    detail: "Your application shows the outcome once they decide.",
  },
] as const;

// What happens after applying. With current set (0-based), that step is marked as the one under way.
export function HiringSteps({
  current,
  className,
}: {
  current?: number;
  className?: string;
}) {
  return (
    <ol className={cn("space-y-3", className)}>
      {steps.map((step, index) => {
        const active = index === current;
        return (
          <li
            key={step.title}
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
                {step.title}
                {active && <span className="sr-only"> (current step)</span>}
              </p>
              <p className="text-muted-foreground">{step.detail}</p>
            </div>
          </li>
        );
      })}
    </ol>
  );
}
