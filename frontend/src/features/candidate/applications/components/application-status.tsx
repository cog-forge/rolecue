import { cn } from "@/lib/utils";
import type { CandidateApplicationStatus } from "../schemas/candidate-application-schema";
import { statusLabel } from "../utils/application-text";

// A calm tone per status; "Not selected" stays neutral rather than alarming.
const toneClass: Record<CandidateApplicationStatus, string> = {
  pending:
    "bg-rolecue-brand/8 text-rolecue-brand ring-rolecue-brand/20 dark:text-sky-300",
  interview_eligible:
    "bg-amber-50 text-amber-900 ring-amber-600/20 dark:bg-amber-500/10 dark:text-amber-200",
  interviewed: "bg-muted text-foreground ring-border",
  approved:
    "bg-emerald-50 text-emerald-900 ring-emerald-600/20 dark:bg-emerald-500/10 dark:text-emerald-200",
  rejected: "bg-muted text-muted-foreground ring-border",
};
const dotClass: Record<CandidateApplicationStatus, string> = {
  pending: "bg-rolecue-brand dark:bg-sky-300",
  interview_eligible: "bg-amber-500",
  interviewed: "bg-foreground/60",
  approved: "bg-emerald-600 dark:bg-emerald-400",
  rejected: "bg-muted-foreground",
};

// Status is always text plus a dot, never colour alone.
export function CandidateApplicationStatusBadge({
  status,
  className,
}: {
  status: CandidateApplicationStatus;
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
        className={cn("size-1.5 shrink-0 rounded-full", dotClass[status])}
      />
      {statusLabel[status]}
    </span>
  );
}
