import { cn } from "@/lib/utils";
import type { PostingStatus as Status } from "../schemas/job-posting-schema";

// A pending posting is either still being prepared by the Recruiter or waiting in the Admin
// queue, so the label comes from the submission as well as the enum.
type Stage =
  | "preparing"
  | "ready"
  | "pending"
  | "open"
  | "intake_closed"
  | "closed"
  | "rejected";

const stageText: Record<Stage, string> = {
  preparing: "Preparing",
  ready: "Ready to submit",
  pending: "Pending review",
  open: "Open",
  intake_closed: "Intake closed",
  closed: "Closed",
  rejected: "Rejected",
};
// One pale tint with dark text of the same hue per stage; blue marks the stage that waits on
// the Recruiter, amber the one that waits on an administrator.
const toneClass: Record<Stage, string> = {
  preparing: "bg-muted text-foreground/80 ring-border",
  ready:
    "bg-rolecue-brand/8 text-rolecue-brand ring-rolecue-brand/20 dark:text-sky-300",
  pending:
    "bg-amber-50 text-amber-900 ring-amber-600/20 dark:bg-amber-500/10 dark:text-amber-200",
  open: "bg-emerald-50 text-emerald-900 ring-emerald-600/20 dark:bg-emerald-500/10 dark:text-emerald-200",
  intake_closed: "bg-muted text-foreground/80 ring-border",
  closed: "bg-muted text-muted-foreground ring-border",
  rejected: "bg-destructive/8 text-destructive ring-destructive/20",
};
const dotClass: Record<Stage, string> = {
  preparing: "bg-muted-foreground",
  ready: "bg-rolecue-brand dark:bg-sky-300",
  pending: "bg-amber-500",
  open: "bg-emerald-600 dark:bg-emerald-400",
  intake_closed: "bg-foreground/60",
  closed: "bg-muted-foreground/60",
  rejected: "bg-destructive",
};

function stageOf(status: Status, ready: boolean, submitted: boolean): Stage {
  if (status !== "pending" || submitted) return status;
  return ready ? "ready" : "preparing";
}

export function postingStatusLabel(status: Status) {
  return stageText[status];
}

// Status is always text plus a dot, never colour alone (team design rule).
export function PostingStatus({
  status,
  className,
  questionBankReady,
  reviewSubmitted,
}: {
  status: Status;
  className?: string;
  questionBankReady: boolean;
  reviewSubmitted: boolean;
}) {
  const stage = stageOf(status, questionBankReady, reviewSubmitted);
  return (
    <span
      className={cn(
        "inline-flex h-6 flex-none items-center gap-1.5 rounded-full px-2.5 text-xs font-medium whitespace-nowrap ring-1 ring-inset",
        toneClass[stage],
        className,
      )}
    >
      <span
        aria-hidden
        className={cn("size-1.5 shrink-0 rounded-full", dotClass[stage])}
      />
      {stageText[stage]}
    </span>
  );
}
