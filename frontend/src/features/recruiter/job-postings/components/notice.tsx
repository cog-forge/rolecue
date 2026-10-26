import type { ReactNode } from "react";
import { CircleAlert, Info, Lock, TriangleAlert } from "lucide-react";
import { cn } from "@/lib/utils";

type Tone = "info" | "warning" | "danger" | "locked";

const toneClass: Record<Tone, string> = {
  info: "border-rolecue-brand/15 bg-rolecue-brand/[0.04]",
  warning:
    "border-amber-500/30 bg-amber-50 dark:border-amber-400/30 dark:bg-amber-500/10",
  danger: "border-destructive/30 bg-destructive/5",
  locked: "border-border bg-muted/60",
};
const iconClass: Record<Tone, string> = {
  info: "text-rolecue-brand dark:text-sky-300",
  warning: "text-amber-600 dark:text-amber-400",
  danger: "text-destructive",
  locked: "text-muted-foreground",
};
const icons = {
  info: Info,
  warning: TriangleAlert,
  danger: CircleAlert,
  locked: Lock,
};

// A tinted panel for state the Recruiter must notice: the tone comes with an icon and words, never
// colour alone.
export function Notice({
  tone,
  title,
  children,
  role,
  className,
}: {
  tone: Tone;
  title?: string;
  children?: ReactNode;
  role?: "status" | "alert";
  className?: string;
}) {
  const Icon = icons[tone];
  return (
    <div
      role={role}
      className={cn(
        "flex gap-3 rounded-lg border px-4 py-3 text-sm leading-relaxed",
        toneClass[tone],
        className,
      )}
    >
      <Icon
        className={cn("mt-0.5 size-4 shrink-0", iconClass[tone])}
        aria-hidden
      />
      <div className="min-w-0 space-y-1 [overflow-wrap:anywhere]">
        {title && <h2 className="font-semibold">{title}</h2>}
        {children}
      </div>
    </div>
  );
}
