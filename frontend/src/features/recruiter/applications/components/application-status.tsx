import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import type { ApplicationStatus as Status } from "../schemas/application-schema";

const toneClass: Record<Status, string> = {
  pending:
    "bg-rolecue-brand/8 text-rolecue-brand ring-rolecue-brand/20 dark:text-sky-300",
  interview_eligible:
    "bg-amber-50 text-amber-800 ring-amber-600/20 dark:bg-amber-500/10 dark:text-amber-300",
  interviewed: "bg-muted text-foreground ring-border",
  approved:
    "bg-emerald-50 text-emerald-800 ring-emerald-600/20 dark:bg-emerald-500/10 dark:text-emerald-300",
  rejected: "bg-destructive/10 text-destructive ring-destructive/25",
};
const dotClass: Record<Status, string> = {
  pending: "bg-rolecue-brand dark:bg-sky-300",
  interview_eligible: "bg-amber-500",
  interviewed: "bg-foreground/60",
  approved: "bg-emerald-600 dark:bg-emerald-400",
  rejected: "bg-destructive",
};

// Status is always text plus a dot, never colour alone (team design rule).
export function ApplicationStatus({
  status,
  className,
}: {
  status: Status;
  className?: string;
}) {
  const t = useTranslations("recruiter.applications.status");
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
      {t(status)}
    </span>
  );
}
