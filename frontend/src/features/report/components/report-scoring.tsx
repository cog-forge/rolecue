"use client";

import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { useFormatters } from "@/lib/i18n/use-formatters";
import type {
  GradedQuestion,
  Report,
  ReportCriterion,
  ReportMetric,
  ReportQuestion,
} from "../schemas/report-schema";
import {
  formatScore,
  overallFormula,
  uncoveredMetrics,
  weightShare,
} from "../utils/scoring";

// The rubric score explanation (KAN-337), shared by the Candidate's practice report and the Recruiter's
// application result. It reads only what the authorized report returned and renders the chain the server
// calculated: criterion points -> question metric -> session metric -> weighted overall. It never averages
// question totals and never shows an expected answer (the API does not return one).

type Scored = Pick<
  Report,
  "overall_score" | "scoring" | "breakdown" | "graded_questions"
>;

/** Overall with its outcome and formula, then the session metrics with their weights. */
export function ScoringSummary({ report }: { report: Scored }) {
  const t = useTranslations("candidate.report.scoring");
  const fmt = useFormatters();
  const { scoring } = report;
  if (!scoring) return null;
  const breakdown = report.breakdown ?? [];
  const legacy = scoring.method === "legacy";
  const overall = report.overall_score;
  const formula = overallFormula(report, fmt.number);
  const missing = uncoveredMetrics(breakdown);

  return (
    <section
      aria-labelledby="scoring-title"
      data-testid="scoring-summary"
      className="space-y-5"
    >
      <div className="space-y-3">
        <div className="flex flex-wrap items-center gap-3">
          <h2 id="scoring-title" className="text-lg font-semibold">
            {t("overall")}
          </h2>
          {scoring.outcome ? (
            <Badge
              variant={scoring.outcome === "withheld" ? "secondary" : "outline"}
              data-testid="scoring-outcome"
            >
              {t(`outcome.${scoring.outcome}`)}
            </Badge>
          ) : null}
        </div>
        {overall !== null ? (
          <p
            className="text-4xl font-semibold tabular-nums"
            data-testid="overall-score"
          >
            {t("outOf100", { score: formatScore(overall, fmt.number) })}
          </p>
        ) : (
          <div className="rounded-lg border border-dashed p-4">
            <p className="font-medium" data-testid="overall-withheld">
              {t("withheld.title")}
            </p>
            {scoring.withheld_reason ? (
              <p className="mt-1 text-sm text-muted-foreground">
                {t(`withheld.${scoring.withheld_reason}`, {
                  metrics: fmt.list(missing.map((metric) => metric.name)),
                })}
              </p>
            ) : null}
          </div>
        )}
        {legacy ? (
          <p className="text-sm text-muted-foreground">{t("legacy")}</p>
        ) : (
          <>
            {scoring.outcome === "partial" ? (
              <p className="text-sm text-muted-foreground">
                {t("partial", {
                  graded: fmt.number(scoring.graded_questions),
                  assigned: fmt.number(scoring.assigned_questions),
                })}
              </p>
            ) : null}
            {formula ? (
              <div className="rounded-lg bg-muted/50 p-3 text-sm">
                <p className="font-medium">{t("formulaTitle")}</p>
                <p className="mt-1 text-muted-foreground">{t("formulaRule")}</p>
                <p
                  className="mt-2 font-mono text-xs tabular-nums [overflow-wrap:anywhere] sm:text-sm"
                  data-testid="overall-formula"
                >
                  {formula}
                </p>
              </div>
            ) : null}
          </>
        )}
      </div>

      <div className="space-y-3">
        <div>
          <h3 className="font-medium">{t("metrics")}</h3>
          {!legacy ? (
            <p className="text-sm text-muted-foreground">{t("metricsRule")}</p>
          ) : null}
        </div>
        <ul className="space-y-4" data-testid="metric-list">
          {breakdown.map((metric) => (
            <MetricRow
              key={metric.metric_id}
              metric={metric}
              total={scoring.weight_total}
              legacy={legacy}
            />
          ))}
        </ul>
      </div>

      {!legacy ? (
        <p className="text-sm text-muted-foreground">
          {t("graded", {
            graded: fmt.number(scoring.graded_questions),
            assigned: fmt.number(scoring.assigned_questions),
          })}{" "}
          {t("questionTotalNote")}
        </p>
      ) : null}
      <p className="text-xs text-muted-foreground">{t("uncalibrated")}</p>
    </section>
  );
}

function MetricRow({
  metric,
  total,
  legacy,
}: {
  metric: ReportMetric;
  total: number;
  legacy: boolean;
}) {
  const t = useTranslations("candidate.report.scoring");
  const fmt = useFormatters();
  const weight = metric.weight ?? 0;
  return (
    <li className="space-y-2">
      <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1 text-sm">
        <span className="font-medium [overflow-wrap:anywhere]">
          {metric.name}
        </span>
        <span className="tabular-nums">
          {metric.score === null ? (
            <span className="text-muted-foreground">{t("noEvidence")}</span>
          ) : (
            t("outOf100", { score: formatScore(metric.score, fmt.number) })
          )}
        </span>
      </div>
      {metric.score !== null ? (
        <div
          role="progressbar"
          aria-label={metric.name}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={metric.score}
          aria-valuetext={t("outOf100", {
            score: formatScore(metric.score, fmt.number),
          })}
          className="h-2 w-full overflow-hidden rounded-full bg-muted"
        >
          <div
            className="h-full rounded-full bg-workspace-accent"
            style={{ width: `${metric.score}%` }}
          />
        </div>
      ) : (
        <div
          aria-hidden="true"
          className="h-2 w-full rounded-full border border-dashed"
        />
      )}
      {!legacy ? (
        <p className="text-xs text-muted-foreground tabular-nums">
          {weight > 0
            ? t("weight", {
                share: fmt.percent(weightShare(weight, total)),
              })
            : t("notWeighted")}
          {" · "}
          {t("questionCount", { count: metric.question_count ?? 0 })}
        </p>
      ) : metric.evidence ? (
        <p className="text-xs text-muted-foreground [overflow-wrap:anywhere]">
          <span className="font-medium">{t("evidence")}: </span>
          <q>{metric.evidence}</q>
        </p>
      ) : null}
    </li>
  );
}

/**
 * How one core question was graded, as a disclosure: closed it says the points and the reference total;
 * open it lists the question metrics and each criterion with its points, quote and feedback.
 */
export function QuestionGrading({ question }: { question: GradedQuestion }) {
  const t = useTranslations("candidate.report.scoring");
  const fmt = useFormatters();
  if (question.status === "not_gradable")
    return (
      <p
        className="text-sm text-muted-foreground"
        data-testid="question-not-gradable"
      >
        {t("notGradable")}
      </p>
    );
  if (question.status === "not_answered")
    return <p className="text-sm text-muted-foreground">{t("notAnswered")}</p>;
  return (
    <details
      className="group rounded-lg border bg-background"
      data-testid="question-grading"
    >
      <summary className="flex min-h-11 cursor-pointer list-none flex-wrap items-center gap-x-3 gap-y-1 rounded-lg px-3 py-2 text-sm focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none [&::-webkit-details-marker]:hidden">
        <span
          aria-hidden="true"
          className="text-muted-foreground transition-transform group-open:rotate-90 motion-reduce:transition-none"
        >
          ▸
        </span>
        <span className="font-medium">{t("grading")}</span>
        <span className="tabular-nums">
          {t("points", {
            points: fmt.number(question.earned),
            max: fmt.number(question.max),
          })}
        </span>
        {question.total !== null ? (
          <span className="text-muted-foreground tabular-nums">
            {t("questionTotal", {
              total: formatScore(question.total, fmt.number),
            })}
          </span>
        ) : null}
      </summary>
      <div className="space-y-4 border-t px-3 py-3 text-sm">
        {question.rubric_name ? (
          <p className="text-muted-foreground [overflow-wrap:anywhere]">
            {t("rubric", { name: question.rubric_name })}
          </p>
        ) : null}
        {question.metrics.length > 0 ? (
          <div>
            <p className="font-medium">{t("questionMetrics")}</p>
            <ul className="mt-1 flex flex-wrap gap-2">
              {question.metrics.map((metric) => (
                <li
                  key={metric.metric_id}
                  className="rounded-md bg-muted px-2 py-1 text-xs tabular-nums"
                >
                  {t("questionMetric", {
                    name: metric.name,
                    earned: fmt.number(metric.earned),
                    max: fmt.number(metric.max),
                    score: formatScore(metric.score, fmt.number),
                  })}
                </li>
              ))}
            </ul>
          </div>
        ) : null}
        <ol className="space-y-3">
          {question.criteria.map((criterion) => (
            <CriterionItem key={criterion.criterion_id} criterion={criterion} />
          ))}
        </ol>
      </div>
    </details>
  );
}

function CriterionItem({ criterion }: { criterion: ReportCriterion }) {
  const t = useTranslations("candidate.report.scoring");
  const fmt = useFormatters();
  const name =
    criterion.name || t("criterionFallback", { metric: criterion.metric_name });
  return (
    <li className="space-y-1" data-testid="criterion">
      <div className="flex flex-wrap items-baseline justify-between gap-x-3">
        <span className="font-medium [overflow-wrap:anywhere]">{name}</span>
        <span className="tabular-nums">
          {t("points", {
            points: fmt.number(criterion.points),
            max: fmt.number(criterion.max_points),
          })}
        </span>
      </div>
      <p className="text-xs text-muted-foreground">{criterion.metric_name}</p>
      {criterion.evidence ? (
        <p className="text-muted-foreground [overflow-wrap:anywhere]">
          <span className="font-medium text-foreground">{t("evidence")}: </span>
          <q>{criterion.evidence}</q>
        </p>
      ) : (
        <p className="text-muted-foreground">{t("noQuote")}</p>
      )}
      {criterion.feedback ? (
        <p className="text-muted-foreground [overflow-wrap:anywhere]">
          <span className="font-medium text-foreground">{t("why")}: </span>
          {criterion.feedback}
        </p>
      ) : null}
    </li>
  );
}

/** Every assigned core question with its text (from the answered turns) and its grading. */
export function GradedQuestionList({
  graded,
  questions,
}: {
  graded: readonly GradedQuestion[];
  questions: readonly ReportQuestion[] | null;
}) {
  const t = useTranslations("candidate.report.scoring");
  const fmt = useFormatters();
  if (graded.length === 0) return null;
  return (
    <section aria-labelledby="graded-questions-title" className="space-y-3">
      <h3 id="graded-questions-title" className="font-medium">
        {t("byQuestion")}
      </h3>
      <ol className="space-y-4">
        {graded.map((item) => {
          const core = questions?.find(
            (turn) =>
              turn.kind === "core" &&
              turn.question_number === item.question_number,
          );
          return (
            <li key={item.question_number} className="space-y-2">
              <p className="text-sm font-medium [overflow-wrap:anywhere]">
                <span className="text-muted-foreground tabular-nums">
                  {t("questionLabel", {
                    number: fmt.number(item.question_number),
                  })}
                </span>
                {core ? ` ${core.question}` : null}
              </p>
              <QuestionGrading question={item} />
            </li>
          );
        })}
      </ol>
    </section>
  );
}
