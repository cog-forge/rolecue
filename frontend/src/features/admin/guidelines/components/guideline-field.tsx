import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export function GuidelineField({
  name,
  legend,
  help,
  children,
  className,
}: {
  name: string;
  legend: string;
  help?: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <fieldset
      className={cn("min-w-0 space-y-3", className)}
      aria-describedby={help ? `${name}-help` : undefined}
    >
      <legend id={`${name}-legend`} className="mb-3 text-sm font-medium">
        {legend}
      </legend>
      {children}
      {help && (
        <p
          id={`${name}-help`}
          className="text-xs leading-relaxed text-muted-foreground"
        >
          {help}
        </p>
      )}
    </fieldset>
  );
}
