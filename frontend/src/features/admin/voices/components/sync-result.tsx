import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import type { SyncVoicesResult } from "../schemas/voice-schema";
import { PanelReveal } from "./panel-reveal";

// What a sync did, kept on screen until dismissed so the Admin can read the numbers.
export function SyncResult({
  result,
  onDismiss,
}: {
  result: SyncVoicesResult;
  onDismiss: () => void;
}) {
  const t = useTranslations("admin.voices.sync");
  const stats = [
    { key: "added", label: t("added"), value: result.added },
    { key: "updated", label: t("updated"), value: result.updated },
    { key: "deactivated", label: t("deactivated"), value: result.deactivated },
    { key: "unchanged", label: t("unchanged"), value: result.unchanged },
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
            {t("result", { count: result.fetched, provider: result.provider })}
          </p>
          <Button
            variant="ghost"
            className="h-10 rounded-full px-4"
            onClick={onDismiss}
          >
            {t("dismiss")}
          </Button>
        </div>
        <dl className="grid grid-cols-2 gap-2 sm:grid-cols-4">
          {stats.map((stat) => (
            <div key={stat.key} className="rounded-lg bg-muted px-3 py-2">
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
