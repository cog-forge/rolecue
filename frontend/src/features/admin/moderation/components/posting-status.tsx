import { cn } from "@/lib/utils";
import type { PostingStatus as Status } from "../schemas/moderation-schema";

const statusText: Record<Status, string> = {
  pending: "Pending review",
  open: "Approved",
  intake_closed: "Intake closed",
  closed: "Closed",
  rejected: "Rejected",
};
const dotClass: Record<Status, string> = {
  pending: "bg-workspace-accent",
  open: "bg-foreground",
  intake_closed: "bg-muted-foreground",
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
        "inline-flex items-center gap-1.5 text-sm whitespace-nowrap",
        className,
      )}
    >
      <span
        aria-hidden
        className={cn("size-2 shrink-0 rounded-full", dotClass[status])}
      />
      {statusText[status]}
    </span>
  );
}
