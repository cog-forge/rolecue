import { useTranslations } from "next-intl";
import { CircleCheck, TriangleAlert } from "lucide-react";
import { coverageBlocks, type Coverage } from "@/lib/api/coverage-schema";
import { cn } from "@/lib/utils";

// How far the bank is from covering every metric, in the colour of the verdict: green when it passes, amber
// when most metrics are covered, red when few are.
type Tone = "ok" | "partial" | "poor";
const toneOf = (coverage: Coverage): Tone =>
  coverage.passes
    ? "ok"
    : coverage.covered / Math.max(coverage.total, 1) >= 0.6
      ? "partial"
      : "poor";
const fill: Record<Tone, string> = {
  ok: "bg-emerald-500",
  partial: "bg-amber-500",
  poor: "bg-destructive",
};
const panel: Record<Tone, string> = {
  ok: "",
  partial:
    "border-amber-500/30 bg-amber-50 dark:border-amber-400/30 dark:bg-amber-500/10",
  poor: "border-destructive/30 bg-destructive/5",
};

// One segment per metric, filled for each one covered. A meter, not a control: the Recruiter changes the
// bank, never the bar.
function CoverageMeter({ coverage, tone }: { coverage: Coverage; tone: Tone }) {
  const t = useTranslations("common.coverage");
  return (
    <div className="flex items-center gap-3">
      <div
        role="meter"
        aria-label={t("title")}
        aria-valuemin={0}
        aria-valuemax={coverage.total}
        aria-valuenow={coverage.covered}
        aria-valuetext={t("summary", {
          covered: coverage.covered,
          total: coverage.total,
        })}
        data-tone={tone}
        className="flex h-2.5 min-w-0 flex-1 gap-1"
      >
        {Array.from({ length: coverage.total }, (_, index) => (
          <span
            key={index}
            aria-hidden
            className={cn(
              "h-full min-w-0 flex-1 rounded-full transition-colors duration-300 motion-reduce:transition-none",
              index < coverage.covered ? fill[tone] : "bg-muted-foreground/20",
            )}
          />
        ))}
      </div>
      <span className="shrink-0 text-sm font-semibold tabular-nums">
        {t("count", { covered: coverage.covered, total: coverage.total })}
      </span>
    </div>
  );
}

// Says whether a posting's question bank measures every metric it is scored on (KAN-377): a segmented meter,
// the metrics nobody measures as chips, and one short line on what to do. It shows nothing when there was
// nothing to assess. A passing bank gets the full green meter, so the warning visibly goes away once fixed.
export function MetricCoverage({
  coverage,
  audience,
  className,
}: {
  coverage: Coverage | null | undefined;
  audience: "recruiter" | "admin";
  className?: string;
}) {
  const t = useTranslations("common.coverage");
  if (!coverage || !coverage.assessed) return null;
  const tone = toneOf(coverage);
  if (!coverageBlocks(coverage))
    return (
      <div
        role="status"
        data-coverage="ok"
        className={cn("space-y-1.5", className)}
      >
        <p className="flex items-center gap-2 text-sm font-medium">
          <CircleCheck
            className="size-4 shrink-0 text-emerald-600 dark:text-emerald-400"
            aria-hidden
          />
          {t("ok")}
        </p>
        <CoverageMeter coverage={coverage} tone={tone} />
      </div>
    );
  const missing = coverage.missing_metrics;
  const rubrics = [
    ...new Set(
      missing.flatMap((metric) =>
        metric.suggested_rubrics.map((rubric) => rubric.name),
      ),
    ),
  ].slice(0, 3);
  return (
    <div
      role="status"
      data-coverage="missing"
      className={cn(
        "space-y-3 rounded-lg border px-4 py-3",
        panel[tone],
        className,
      )}
    >
      <div className="flex items-center gap-2 text-sm font-semibold">
        <TriangleAlert
          className="size-4 shrink-0 text-amber-600 dark:text-amber-400"
          aria-hidden
        />
        <h2>{t("title")}</h2>
      </div>
      <CoverageMeter coverage={coverage} tone={tone} />
      {missing.length > 0 ? (
        <div className="space-y-1.5 text-sm">
          <p className="flex flex-wrap items-center gap-1.5">
            <span className="text-muted-foreground">{t("missing")}</span>
            {missing.map((metric) => (
              <span
                key={metric.id}
                className="rounded-full border bg-background/70 px-2.5 py-0.5 text-xs font-medium [overflow-wrap:anywhere]"
              >
                {metric.name}
              </span>
            ))}
          </p>
          <p className="text-muted-foreground">
            {audience === "recruiter"
              ? rubrics.length > 0
                ? t("hintRecruiter", {
                    count: missing.length,
                    rubrics: rubrics.join(", "),
                  })
                : t("hintRecruiterNoRubric")
              : t("hintAdmin")}
          </p>
        </div>
      ) : (
        <p className="text-sm text-muted-foreground">
          {t("exceeds", {
            limit: coverage.question_limit,
            min: coverage.min_questions,
          })}
        </p>
      )}
    </div>
  );
}
