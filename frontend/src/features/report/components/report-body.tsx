import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { routes } from "@/config/routes";
import type { Report } from "../schemas/report-schema";
import {
  formatInterviewTime,
  reportStatusLabel,
  reportTypeLabel,
  scoreText,
} from "../utils/report-text";

// Renders what the API returned. Metric names and weights come from the server, never from here.
// Null sections (a recruitment report seen by the Candidate) are simply not shown.
export function ReportBody({
  report,
  onCheckAgain,
  checking,
}: {
  report: Report;
  onCheckAgain: () => void;
  checking: boolean;
}) {
  const { type, status, overall_score: score } = report;
  const finished = status === "completed";

  return (
    <div className="space-y-6 duration-300 animate-in fade-in motion-reduce:animate-none">
      <header className="space-y-3">
        <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
          Interview report
        </p>
        <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
          {reportTypeLabel[type]}
        </h1>
        <div className="flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
          <Badge variant="outline">{reportStatusLabel[status]}</Badge>
          <span className="tabular-nums">
            Started{" "}
            <time dateTime={report.started_at}>
              {formatInterviewTime(report.started_at)}
            </time>
          </span>
          {report.completed_at ? (
            <span className="tabular-nums">
              Completed{" "}
              <time dateTime={report.completed_at}>
                {formatInterviewTime(report.completed_at)}
              </time>
            </span>
          ) : null}
        </div>
      </header>

      <StatusNotice
        status={status}
        onCheckAgain={onCheckAgain}
        checking={checking}
      />

      {finished ? (
        <Card>
          <CardHeader>
            <CardTitle>
              <h2 className="text-lg font-semibold">Overall score</h2>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <p className="text-4xl font-semibold tabular-nums">
              {scoreText(score)}
            </p>
            {report.summary ? (
              <div className="space-y-2">
                <h3 className="font-medium">Summary</h3>
                <p className="text-sm leading-relaxed whitespace-pre-line text-muted-foreground">
                  {report.summary}
                </p>
              </div>
            ) : null}
          </CardContent>
        </Card>
      ) : null}

      {finished && report.breakdown ? (
        <Card>
          <CardHeader>
            <CardTitle>
              <h2 className="text-lg font-semibold">Breakdown</h2>
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
                    <span className="tabular-nums">{metric.score} / 100</span>
                  </div>
                  <div
                    role="progressbar"
                    aria-label={metric.name}
                    aria-valuemin={0}
                    aria-valuemax={100}
                    aria-valuenow={metric.score}
                    aria-valuetext={`${metric.score} out of 100`}
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
              <h2 className="text-lg font-semibold">Questions</h2>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <ol className="space-y-6">
              {report.questions.map((item) => (
                <li
                  key={item.position}
                  className="space-y-2 border-t pt-4 first:border-t-0 first:pt-0"
                >
                  <h3 className="font-medium [overflow-wrap:anywhere]">
                    <span className="text-muted-foreground tabular-nums">
                      {item.position}.
                    </span>{" "}
                    {item.question}
                  </h3>
                  <p className="text-sm text-muted-foreground [overflow-wrap:anywhere]">
                    <span className="font-medium text-foreground">
                      Answer:{" "}
                    </span>
                    {item.answer ?? "No answer recorded."}
                  </p>
                  {item.feedback ? (
                    <p className="text-sm text-muted-foreground [overflow-wrap:anywhere]">
                      <span className="font-medium text-foreground">
                        Feedback:{" "}
                      </span>
                      {item.feedback}
                    </p>
                  ) : null}
                </li>
              ))}
            </ol>
          </CardContent>
        </Card>
      ) : null}

      {finished && type === "recruitment" && !report.breakdown ? (
        <p className="text-sm text-muted-foreground">
          Detailed feedback for recruitment interviews is not shown here.
        </p>
      ) : null}

      <Button asChild variant="outline" className="h-11">
        <Link href={routes.history}>Back to interview history</Link>
      </Button>
    </div>
  );
}

function StatusNotice({
  status,
  onCheckAgain,
  checking,
}: {
  status: Report["status"];
  onCheckAgain: () => void;
  checking: boolean;
}) {
  if (status === "completed") return null;

  const copy = {
    in_progress: {
      title: "This interview isn't finished yet",
      text: "The report appears here once the interview is completed.",
    },
    pending: {
      title: "Your interview is being evaluated",
      text: "Scores and feedback will appear here when the evaluation is done.",
    },
    failed: {
      title: "We couldn't evaluate this interview",
      text: "No scores were saved for it, so there is nothing to show yet.",
    },
  }[status];

  return (
    <section
      aria-labelledby="report-status-title"
      className="rounded-xl border bg-muted/40 p-5"
    >
      <h2 id="report-status-title" className="font-semibold">
        {copy.title}
      </h2>
      <p className="mt-2 text-sm text-muted-foreground">{copy.text}</p>
      {status === "pending" || status === "in_progress" ? (
        <Button
          variant="outline"
          className="mt-4 h-11 bg-background"
          disabled={checking}
          onClick={onCheckAgain}
        >
          Check again
        </Button>
      ) : null}
    </section>
  );
}
