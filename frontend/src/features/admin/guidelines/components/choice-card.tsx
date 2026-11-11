import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export function ChoiceCard({
  htmlFor,
  title,
  description,
  control,
  icon,
  className,
}: {
  htmlFor: string;
  title: ReactNode;
  description?: ReactNode;
  control: ReactNode;
  icon?: ReactNode;
  className?: string;
}) {
  return (
    <label
      htmlFor={htmlFor}
      className={cn(
        "group/choice flex min-h-11 cursor-pointer items-center gap-2.5 rounded-lg border bg-card px-3 py-3 transition-[border-color,background-color,box-shadow] duration-[var(--duration-quick)] has-[:focus-visible]:ring-3 has-[:focus-visible]:ring-ring/50 has-[[data-state=checked]]:border-workspace-accent has-[[data-state=checked]]:bg-workspace-accent/5 has-disabled:cursor-not-allowed has-disabled:opacity-60 hover:bg-muted/50 motion-reduce:transition-none",
        className,
      )}
    >
      {icon && (
        <span
          aria-hidden
          className="hidden shrink-0 text-muted-foreground sm:block group-has-[[data-state=checked]]/choice:text-foreground"
        >
          {icon}
        </span>
      )}
      <span className="min-w-0 flex-1">
        <span className="block text-sm font-medium">{title}</span>
        {description && (
          <span className="mt-0.5 block text-xs text-muted-foreground">
            {description}
          </span>
        )}
      </span>
      <span className="shrink-0">{control}</span>
    </label>
  );
}
