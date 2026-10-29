import type { ReactNode } from "react";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { cn } from "@/lib/utils";

// Workspace actions share the same hierarchy: near-black for the one
// primary action, outlined for the rest.
export const pillClass = "h-10 rounded-full px-5";

// The title row of a posting screen: a way back, the noun or object name as the h1, its state
// and dates underneath, and the screen's actions on the right.
export function PageHeader({
  back,
  title,
  meta,
  description,
  actions,
  className,
}: {
  back?: { href: string; label: string };
  title: ReactNode;
  meta?: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  className?: string;
}) {
  return (
    <header className={cn("space-y-3", className)}>
      {back && (
        <Link
          href={back.href}
          className="-ml-1 inline-flex min-h-11 items-center gap-1.5 rounded-md px-1 text-sm text-muted-foreground transition-colors duration-150 outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          <ArrowLeft className="size-4" aria-hidden />
          {back.label}
        </Link>
      )}
      <div className="flex flex-col gap-x-6 gap-y-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0 flex-1 space-y-2">
          <h1 className="text-2xl font-semibold tracking-tight text-balance [overflow-wrap:anywhere] sm:text-[1.75rem] sm:leading-tight">
            {title}
          </h1>
          {meta && (
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 text-sm text-muted-foreground">
              {meta}
            </div>
          )}
          {description && (
            <p className="max-w-[68ch] text-sm leading-relaxed text-muted-foreground">
              {description}
            </p>
          )}
        </div>
        {actions && (
          <div className="flex flex-wrap items-center gap-2">{actions}</div>
        )}
      </div>
    </header>
  );
}
