import type { ReactNode } from "react";
import { History } from "lucide-react";
import { cn } from "@/lib/utils";
import { useFormatters } from "@/lib/i18n/use-formatters";
import type { UnfinishedPractice } from "../api/setup-catalog";

export function PracticeProgress({ session }: { session: UnfinishedPractice }) {
  const fmt = useFormatters();
  return (
    <>
      Started{" "}
      <time dateTime={session.startedAt}>
        {fmt.relative(session.startedAt)}
      </time>
      {" · "}
      <span className="tabular-nums">
        {session.answeredCount} of {session.questionCount}
      </span>{" "}
      questions answered
    </>
  );
}

// The resume path looks different from a new start on purpose: the brand surface instead of a
// neutral card, no price, and the reason it costs nothing. The server would return this session
// anyway, so there is nothing new to configure.
export function ResumeCallout({
  title,
  details,
  note,
  children,
  className,
}: {
  title: string;
  details?: ReactNode;
  note: string;
  children?: ReactNode;
  className?: string;
}) {
  return (
    <section
      aria-label="Unfinished interview"
      className={cn(
        "space-y-4 rounded-xl bg-rolecue-hero-surface/60 p-4 duration-(--duration-slow) ease-(--ease-smooth-out) animate-in fade-in slide-in-from-bottom-1 motion-reduce:animate-none sm:p-5",
        className,
      )}
    >
      <div className="flex items-start gap-3">
        <History
          aria-hidden="true"
          className="mt-0.5 size-5 shrink-0 text-rolecue-brand"
        />
        <div className="min-w-0 space-y-1">
          <h2 className="font-semibold [overflow-wrap:anywhere]">{title}</h2>
          {details && (
            <p className="text-sm text-muted-foreground">{details}</p>
          )}
          <p className="text-sm">{note}</p>
        </div>
      </div>
      {children}
    </section>
  );
}
