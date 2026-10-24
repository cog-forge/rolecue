import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

// Settings row: a label column beside a tonal panel; sections are split by one rule.
export function SecuritySection({
  id,
  title,
  description,
  children,
}: {
  id: string;
  title: string;
  description: string;
  children: ReactNode;
}) {
  return (
    <section
      aria-labelledby={id}
      className="grid scroll-mt-24 gap-4 border-t py-8 md:grid-cols-[minmax(0,16rem)_minmax(0,1fr)] md:gap-10"
    >
      <header className="space-y-1.5 md:pt-3">
        <h2 id={id} className="text-base font-semibold tracking-tight">
          {title}
        </h2>
        <p className="text-sm leading-6 text-muted-foreground">{description}</p>
      </header>
      <div className="min-w-0 space-y-3">{children}</div>
    </section>
  );
}

export function SecurityPanel({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "divide-y divide-border/70 rounded-2xl bg-muted px-4 sm:px-5",
        className,
      )}
    >
      {children}
    </div>
  );
}

export function SecurityRow({
  icon,
  title,
  status,
  description,
  action,
  children,
}: {
  icon?: ReactNode;
  title: ReactNode;
  status?: ReactNode;
  description?: ReactNode;
  action?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <div className="py-4">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-3">
        <div className="flex min-w-0 flex-1 basis-56 items-start gap-3">
          {icon && (
            <span className="mt-0.5 flex size-5 shrink-0 items-center justify-center text-muted-foreground [&_svg]:size-[18px]">
              {icon}
            </span>
          )}
          <div className="min-w-0 space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <p className="text-sm font-medium">{title}</p>
              {status}
            </div>
            {description && (
              <div className="text-sm leading-6 text-muted-foreground">
                {description}
              </div>
            )}
          </div>
        </div>
        {action && (
          <div className="flex shrink-0 flex-wrap items-center gap-2">
            {action}
          </div>
        )}
      </div>
      {children}
    </div>
  );
}
