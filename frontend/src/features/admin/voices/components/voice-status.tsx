import { cn } from "@/lib/utils";

// Status is always text plus a dot, never colour alone (team design rule). Active is a soft
// pastel green so it doesn't compete with the content; inactive stays neutral.
export function VoiceStatus({ active }: { active: boolean }) {
  return (
    <span
      className={cn(
        "inline-flex h-6 items-center gap-1.5 rounded-full px-2.5 text-xs font-medium whitespace-nowrap",
        active
          ? "bg-emerald-50 text-emerald-700 dark:bg-emerald-400/10 dark:text-emerald-300"
          : "bg-muted text-muted-foreground",
      )}
    >
      <span
        aria-hidden
        className={cn(
          "size-2 shrink-0 rounded-full",
          active ? "bg-emerald-400" : "bg-muted-foreground/60",
        )}
      />
      {active ? "Active" : "Inactive"}
    </span>
  );
}
