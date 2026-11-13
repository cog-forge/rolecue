import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

/**
 * Standard semantic table skeleton matching RoleCue DataTable layout.
 */
export function TableSkeleton({
  rows = 5,
  columns = 4,
  showHeader = true,
  label,
  className,
}: {
  rows?: number;
  columns?: number;
  showHeader?: boolean;
  label?: string;
  className?: string;
}) {
  return (
    <div
      role="status"
      aria-label={label}
      className={cn(
        "min-w-0 overflow-hidden rounded-xl border bg-card/60 shadow-xs",
        className,
      )}
    >
      {showHeader && (
        <div className="flex h-12 items-center gap-4 border-b bg-muted/40 px-5">
          {Array.from({ length: columns }, (_, i) => (
            <Skeleton
              key={`head-${i}`}
              className={cn(
                "h-4 rounded-md",
                i === 0 ? "w-36" : i === 1 ? "w-24" : "w-20",
              )}
            />
          ))}
        </div>
      )}

      <div className="divide-y divide-border/60">
        {Array.from({ length: rows }, (_, rowIdx) => (
          <div
            key={`row-${rowIdx}`}
            className="flex items-center gap-4 px-5 py-4"
          >
            {Array.from({ length: columns }, (_, colIdx) => (
              <Skeleton
                key={`cell-${rowIdx}-${colIdx}`}
                className={cn(
                  "h-5 rounded-md",
                  colIdx === 0
                    ? "w-44"
                    : colIdx === 1
                      ? "w-28"
                      : colIdx === 2
                        ? "w-20"
                        : "w-16",
                )}
              />
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}

/**
 * Grid of card skeletons for dashboard cards, job board cards, avatar items.
 */
export function CardGridSkeleton({
  count = 6,
  className,
}: {
  count?: number;
  className?: string;
}) {
  return (
    <div
      role="status"
      aria-label="Loading cards"
      className={cn("grid gap-4 sm:grid-cols-2 lg:grid-cols-3", className)}
    >
      {Array.from({ length: count }, (_, i) => (
        <div
          key={i}
          className="flex flex-col justify-between rounded-2xl border bg-card/60 p-6 shadow-xs space-y-4"
        >
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <Skeleton className="h-6 w-24 rounded-full" />
              <Skeleton className="size-6 rounded-full" />
            </div>
            <Skeleton className="h-5 w-3/4 rounded-md" />
            <Skeleton className="h-4 w-full rounded-md" />
            <Skeleton className="h-4 w-2/3 rounded-md" />
          </div>

          <div className="pt-2 flex items-center justify-between border-t border-border/50">
            <Skeleton className="h-4 w-20 rounded-md" />
            <Skeleton className="h-8 w-24 rounded-lg" />
          </div>
        </div>
      ))}
      <span className="sr-only">Loading cards...</span>
    </div>
  );
}

/**
 * Stack of list item skeletons (applications, interview history, activities).
 */
export function ListSkeleton({
  count = 4,
  className,
}: {
  count?: number;
  className?: string;
}) {
  return (
    <div
      role="status"
      aria-label="Loading list"
      className={cn("space-y-3", className)}
    >
      {Array.from({ length: count }, (_, i) => (
        <div
          key={i}
          className="flex items-center justify-between rounded-xl border bg-card/60 p-4 shadow-xs"
        >
          <div className="flex items-center gap-3">
            <Skeleton className="size-10 rounded-xl" />
            <div className="space-y-2">
              <Skeleton className="h-4 w-40 rounded-md" />
              <Skeleton className="h-3 w-28 rounded-md" />
            </div>
          </div>
          <div className="flex items-center gap-3">
            <Skeleton className="h-6 w-16 rounded-full" />
            <Skeleton className="size-8 rounded-lg" />
          </div>
        </div>
      ))}
      <span className="sr-only">Loading list...</span>
    </div>
  );
}

/**
 * Detailed view skeleton for single entity pages.
 */
export function DetailSkeleton({ className }: { className?: string }) {
  return (
    <div
      role="status"
      aria-label="Loading detail"
      className={cn("w-full space-y-6", className)}
    >
      {/* Top action / breadcrumb */}
      <div className="flex items-center justify-between">
        <Skeleton className="h-5 w-32 rounded-md" />
        <Skeleton className="h-9 w-28 rounded-xl" />
      </div>

      {/* Hero header block */}
      <div className="rounded-2xl border bg-card/70 p-6 sm:p-8 space-y-4">
        <div className="flex flex-wrap items-center gap-3">
          <Skeleton className="h-7 w-20 rounded-full" />
          <Skeleton className="h-7 w-28 rounded-full" />
        </div>
        <Skeleton className="h-8 w-2/3 rounded-lg sm:h-10" />
        <Skeleton className="h-4 w-full max-w-xl rounded-md" />
      </div>

      {/* Main content grid */}
      <div className="grid gap-6 md:grid-cols-3">
        <div className="rounded-2xl border bg-card/60 p-6 space-y-4 md:col-span-2">
          <Skeleton className="h-6 w-40 rounded-md" />
          <Skeleton className="h-4 w-full rounded-md" />
          <Skeleton className="h-4 w-5/6 rounded-md" />
          <Skeleton className="h-4 w-3/4 rounded-md" />
          <Skeleton className="h-32 w-full rounded-xl mt-4" />
        </div>
        <div className="rounded-2xl border bg-card/60 p-6 space-y-4">
          <Skeleton className="h-6 w-32 rounded-md" />
          <Skeleton className="h-16 w-full rounded-xl" />
          <Skeleton className="h-16 w-full rounded-xl" />
        </div>
      </div>
      <span className="sr-only">Loading detail...</span>
    </div>
  );
}
