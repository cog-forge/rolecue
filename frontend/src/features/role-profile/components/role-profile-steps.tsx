import { Check } from "lucide-react";
import { cn } from "@/lib/utils";

const steps = [
  { id: "input", label: "Add JD" },
  { id: "review", label: "Review role" },
  { id: "confirmed", label: "Confirmed" },
] as const;

export function RoleProfileSteps({
  current,
}: {
  current: (typeof steps)[number]["id"];
}) {
  const active = steps.findIndex((step) => step.id === current);
  return (
    <ol
      aria-label="Role Profile progress"
      className="flex flex-wrap items-center gap-x-5 gap-y-3 text-xs sm:gap-x-8"
    >
      {steps.map((step, index) => (
        <li
          key={step.id}
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
          {step.label}
          <span className="sr-only">
            {index < active
              ? ", complete"
              : index === active
                ? ", current step"
                : ", upcoming"}
          </span>
        </li>
      ))}
    </ol>
  );
}
