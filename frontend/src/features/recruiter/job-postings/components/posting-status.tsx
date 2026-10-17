import { cn } from "@/lib/utils";
import type { PostingStatus as Status } from "../schemas/job-posting-schema";

const statusText: Record<Status, string> = {
  pending: "Pending review",
  open: "Open",
  intake_closed: "Intake closed",
  closed: "Closed",
  rejected: "Rejected",
};
const toneClass: Record<Status, string> = {
  pending:
    "bg-workspace-accent/10 text-workspace-accent ring-workspace-accent/25",
  open: "bg-emerald-50 text-emerald-800 ring-emerald-600/20 dark:bg-emerald-500/10 dark:text-emerald-300",
  intake_closed:
    "bg-amber-50 text-amber-800 ring-amber-600/20 dark:bg-amber-500/10 dark:text-amber-300",
  closed: "bg-muted text-muted-foreground ring-border",
  rejected: "bg-destructive/10 text-destructive ring-destructive/25",
};
const dotClass: Record<Status, string> = {
  pending: "bg-workspace-accent",
  open: "bg-emerald-600 dark:bg-emerald-400",
  intake_closed: "bg-amber-500",
  closed: "bg-muted-foreground",
  rejected: "bg-destructive",
};

export function postingStatusLabel(status: Status) {
  return statusText[status];
}

// Status is always text plus a dot, never colour alone (team design rule).
export function PostingStatus({
  status,
  className,
}: {
  status: Status;
  className?: string;
}) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium whitespace-nowrap ring-1 ring-inset",
        toneClass[status],
        className,
      )}
    >
      <span
        aria-hidden
        className={cn(
          "size-1.5 shrink-0 rounded-full",
          dotClass[status],
          status === "pending" && "animate-pulse motion-reduce:animate-none",
        )}
      />
      {statusText[status]}
    </span>
  );
}
