import { Button } from "@/components/ui/button";
import type { SyncVoicesResult } from "../schemas/voice-schema";
import { PanelReveal } from "./panel-reveal";

export function syncSummary(result: SyncVoicesResult) {
  return `${result.added} added, ${result.updated} updated, ${result.deactivated} deactivated, ${result.unchanged} unchanged.`;
}

// What a sync did, kept on screen until dismissed so the Admin can read the numbers.
export function SyncResult({
  result,
  onDismiss,
}: {
  result: SyncVoicesResult;
  onDismiss: () => void;
}) {
  const stats = [
    { label: "Added", value: result.added },
    { label: "Updated", value: result.updated },
    { label: "Deactivated", value: result.deactivated },
    { label: "Unchanged", value: result.unchanged },
  ];
  return (
    <PanelReveal>
      <div role="status" className="space-y-3 rounded-xl border p-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <p className="flex items-center gap-2 text-sm font-medium">
            <span className="t-success-check" data-state="in">
              <svg
                viewBox="0 0 24 24"
                width="20"
                height="20"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
                aria-hidden
              >
                <path d="M5 12l4 4L19 6" />
              </svg>
            </span>
            Synced {result.fetched} voices from {result.provider}
          </p>
          <Button
            variant="ghost"
            className="h-10 rounded-full px-4"
            onClick={onDismiss}
          >
            Dismiss
          </Button>
        </div>
        <dl className="grid grid-cols-2 gap-2 sm:grid-cols-4">
          {stats.map((stat) => (
            <div key={stat.label} className="rounded-lg bg-muted px-3 py-2">
              <dt className="text-xs text-muted-foreground">{stat.label}</dt>
              <dd className="text-lg font-semibold tabular-nums">
                {stat.value}
              </dd>
            </div>
          ))}
        </dl>
      </div>
    </PanelReveal>
  );
}
