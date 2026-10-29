import { cn } from "@/lib/utils";
import type { CandidateApplication } from "../schemas/candidate-application-schema";

const labels = ["Applied", "CV review", "Interview", "Decision"] as const;
type StepState = "done" | "current" | "ended" | "upcoming";

const stateText: Record<StepState, string> = {
  done: "done",
  current: "in progress",
  ended: "ended here",
  upcoming: "not reached",
};
const barClass: Record<StepState, string> = {
  done: "bg-foreground/70",
  current: "bg-rolecue-brand",
  ended: "bg-destructive",
  upcoming: "bg-muted",
};

// Where the Application stands. A rejection ends at the step it happened: the final decision after
// an interview, the interview window after an approved CV, otherwise the CV review.
export function progressSteps(
  application: Pick<
    CandidateApplication,
    "status" | "interview" | "interview_deadline"
  >,
): StepState[] {
  const { status } = application;
  if (status === "rejected") {
    const end = application.interview
      ? 3
      : application.interview_deadline
        ? 2
        : 1;
    return labels.map((_, i) =>
      i < end ? "done" : i === end ? "ended" : "upcoming",
    );
  }
  const current =
    status === "pending"
      ? 1
      : status === "interview_eligible"
        ? 2
        : status === "interviewed"
          ? 3
          : labels.length;
  return labels.map((_, i) =>
    i < current ? "done" : i === current ? "current" : "upcoming",
  );
}

// Four short bars with labels; each step's state is also spelled out for screen readers.
export function ApplicationProgress({
  application,
  className,
}: {
  application: Pick<
    CandidateApplication,
    "status" | "interview" | "interview_deadline"
  >;
  className?: string;
}) {
  const states = progressSteps(application);
  return (
    <ol
      aria-label="Application progress"
      className={cn("grid grid-cols-4 gap-1.5", className)}
    >
      {labels.map((label, i) => (
        <li
          key={label}
          aria-current={states[i] === "current" ? "step" : undefined}
          className="min-w-0 space-y-1.5"
        >
          <span
            aria-hidden
            className={cn("block h-1 rounded-full", barClass[states[i]])}
          />
          <span
            className={cn(
              "block truncate text-xs",
              states[i] === "upcoming"
                ? "text-muted-foreground"
                : "font-medium text-foreground",
            )}
          >
            {label}
            <span className="sr-only">: {stateText[states[i]]}</span>
          </span>
        </li>
      ))}
    </ol>
  );
}
