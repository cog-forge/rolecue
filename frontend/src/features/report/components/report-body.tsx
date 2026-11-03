import { ReportExport } from "./report-export";
import type { ReactNode } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { routes } from "@/config/routes";
import type { Report, ReportQuestion } from "../schemas/report-schema";
import { groupByQuestion } from "../utils/question-groups";
import { formatInterviewTime, scoreText } from "../utils/report-text";
import { useFormatters } from "@/lib/i18n/use-formatters";

// The tag name in the message becomes a <time> element carrying the machine-readable instant.
function timeTag(iso: string) {
  return {
    time: (chunks: ReactNode) => <time dateTime={iso}>{chunks}</time>,
  };
}

// Renders what the API returned. Metric names and weights come from the server, never from here.
// Null sections (a recruitment report seen by the Candidate) are simply not shown.
export function ReportBody({
  report,
  onCheckAgain,
  checking,
  autoRefresh = false,
}: {
  report: Report;
  onCheckAgain: () => void;
  checking: boolean;
  /** The page refetches by itself while evaluation is pending (bounded). */
  autoRefresh?: boolean;
}) {
  const t = useTranslations("candidate.report");
  const fmt = useFormatters();
  const { type, status, overall_score: score } = report;
  const finished = status === "completed";

  return (
    <div className="space-y-6 duration-300 animate-in fade-in motion-reduce:animate-none">
      <header className="space-y-3">
        <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
          {t("eyebrow")}
        </p>
        <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
          {t(`type.${type}`)}
        </h1>
        <div className="flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
          <Badge variant="outline">{t(`status.${status}`)}</Badge>
          <span className="tabular-nums">
            {t.rich("startedAt", {
              when: formatInterviewTime(report.started_at, fmt),
              ...timeTag(report.started_at),
            })}
          </span>
          {report.completed_at ? (
            <span className="tabular-nums">
              {t.rich("completedAt", {
                when: formatInterviewTime(report.completed_at, fmt),
                ...timeTag(report.completed_at),
              })}
            </span>
          ) : null}
        </div>
      </header>

      <EndNotice report={report} />
      <ReportExport report={report} />

      <StatusNotice
        status={status}
        onCheckAgain={onCheckAgain}
        checking={checking}
        autoRefresh={autoRefresh}
      />

      {finished ? (
        <Card>
          <CardHeader>
            <CardTitle>
              <h2 className="text-lg font-semibold">{t("overallScore")}</h2>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <p className="text-4xl font-semibold tabular-nums">
              {scoreText(score, fmt, t)}
            </p>
            {report.summary ? (
              <div className="space-y-2">
                <h3 className="font-medium">{t("summary")}</h3>
                <p className="text-sm leading-relaxed whitespace-pre-line text-muted-foreground">
                  {report.summary}
                </p>
              </div>
            ) : null}
          </CardContent>
        </Card>
      ) : null}

      {finished && report.breakdown && report.breakdown.length > 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>
              <h2 className="text-lg font-semibold">{t("breakdown")}</h2>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <ul className="space-y-4">
              {report.breakdown.map((metric) => (
                <li key={metric.metric_id} className="space-y-2">
                  <div className="flex items-center justify-between gap-3 text-sm">
                    <span className="font-medium [overflow-wrap:anywhere]">
                      {metric.name}
                    </span>
                    <span className="tabular-nums">
                      {t("score", { score: fmt.number(metric.score) })}
                    </span>
                  </div>
                  <div
                    role="progressbar"
                    aria-label={metric.name}
                    aria-valuemin={0}
                    aria-valuemax={100}
                    aria-valuenow={metric.score}
                    aria-valuetext={t("scoreText", {
                      score: fmt.number(metric.score),
                    })}
                    className="h-2 w-full overflow-hidden rounded-full bg-muted"
                  >
                    <div
                      className="h-full rounded-full bg-workspace-accent"
                      style={{ width: `${metric.score}%` }}
                    />
                  </div>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>
      ) : null}

      {finished && report.questions ? (
        <Card>
          <CardHeader>
            <CardTitle>
              <h2 className="text-lg font-semibold">{t("questions")}</h2>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <ol className="space-y-6">
              {groupByQuestion(report.questions).map(({ core, followUps }) => (
                <li
                  key={core.position}
                  className="space-y-2 border-t pt-4 first:border-t-0 first:pt-0"
                >
                  <h3 className="font-medium [overflow-wrap:anywhere]">
                    <span className="text-muted-foreground tabular-nums">
                      {core.question_number}.
                    </span>{" "}
                    {core.question}
                  </h3>
                  <TurnDetails item={core} />
                  {followUps.length > 0 ? (
                    <ol className="mt-3 space-y-3 border-l-2 pl-4">
                      {followUps.map((item) => (
                        <li key={item.position} className="space-y-2">
                          <h4 className="text-sm font-medium [overflow-wrap:anywhere]">
                            <Badge variant="secondary" className="mr-2">
                              {t("followUp")}
                            </Badge>
                            {item.question}
                          </h4>
                          <TurnDetails item={item} />
                        </li>
                      ))}
                    </ol>
                  ) : null}
                </li>
              ))}
            </ol>
          </CardContent>
        </Card>
      ) : null}

      {finished && type === "recruitment" && !report.breakdown ? (
        <p className="text-sm text-muted-foreground">{t("recruitmentNote")}</p>
      ) : null}

      <Button
        asChild
        variant="outline"
        className="h-auto min-h-11 py-2 whitespace-normal"
      >
        <Link href={routes.history}>{t("backToHistory")}</Link>
      </Button>
    </div>
  );
}

// The Candidate's answer to one question and its feedback once evaluated.
function TurnDetails({ item }: { item: ReportQuestion }) {
  const t = useTranslations("candidate.report");
  return (
    <>
      <p className="text-sm text-muted-foreground [overflow-wrap:anywhere]">
        <span className="font-medium text-foreground">{t("answer")} </span>
        {item.answer ?? t("noAnswer")}
      </p>
      {item.feedback ? (
        <p className="text-sm text-muted-foreground [overflow-wrap:anywhere]">
          <span className="font-medium text-foreground">{t("feedback")} </span>
          {item.feedback}
        </p>
      ) : null}
    </>
  );
}

// An interview the Candidate left, or that ended without activity, says so, with how many core
// questions were answered. Unanswered questions have no entry in the list and are not scored.
function EndNotice({ report }: { report: Report }) {
  const t = useTranslations("candidate.report.end");
  const fmt = useFormatters();
  const reason = report.end_reason;
  if (reason !== "left" && reason !== "abandoned") return null;
  const total = report.question_count;
  const answered = report.questions
    ? report.questions.filter(
        (item) => item.kind === "core" && item.answer !== null,
      ).length
    : null;
  return (
    <section
      aria-labelledby="report-end-title"
      data-testid="report-end-notice"
      className="rounded-xl border bg-muted/40 p-5"
    >
      <h2 id="report-end-title" className="font-semibold">
        {t(`${reason}Title`)}
      </h2>
      <p className="mt-2 text-sm text-muted-foreground">{t(`${reason}Text`)}</p>
      {answered !== null && total !== undefined && total > 0 ? (
        <p className="mt-2 text-sm text-muted-foreground tabular-nums">
          {t("answered", {
            answered: fmt.number(answered),
            total: fmt.number(total),
            unanswered: Math.max(total - answered, 0),
          })}
        </p>
      ) : null}
    </section>
  );
}

function StatusNotice({
  status,
  onCheckAgain,
  checking,
  autoRefresh,
}: {
  status: Report["status"];
  onCheckAgain: () => void;
  checking: boolean;
  autoRefresh: boolean;
}) {
  const t = useTranslations("candidate.report");
  if (status === "completed") return null;

  return (
    <section
      aria-labelledby="report-status-title"
      className="rounded-xl border bg-muted/40 p-5"
    >
      <h2 id="report-status-title" className="font-semibold">
        {t(`notice.${status}.title`)}
      </h2>
      <p className="mt-2 text-sm text-muted-foreground">
        {t(`notice.${status}.text`)}
      </p>
      {status === "pending" ? (
        <p role="status" className="mt-2 text-sm text-muted-foreground">
          {autoRefresh ? t("autoRefresh") : t("autoRefreshStopped")}
        </p>
      ) : null}
      {status === "pending" || status === "in_progress" ? (
        <Button
          variant="outline"
          className="mt-4 h-auto min-h-11 py-2 whitespace-normal bg-background"
          disabled={checking}
          onClick={onCheckAgain}
        >
          {t("checkAgain")}
        </Button>
      ) : null}
    </section>
  );
}
