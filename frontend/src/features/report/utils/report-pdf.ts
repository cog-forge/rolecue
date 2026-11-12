import type { Report } from "../schemas/report-schema";
import { groupByQuestion } from "./question-groups";
import type { ReportTranslator } from "./report-text";
import {
  formatScore,
  overallFormula,
  uncoveredMetrics,
  weightShare,
} from "./scoring";

function escape(text: string) {
  return text.replace(/[&<>"']/g, (char) => {
    const entities: Record<string, string> = {
      "&": "&amp;",
      "<": "&lt;",
      ">": "&gt;",
      '"': "&quot;",
      "'": "&#39;",
    };
    return entities[char];
  });
}

// Strings of the score explanation (candidate.report.scoring), passed in so this module stays free of hooks.
export type ScoringTranslator = (
  key: string,
  values?: Record<string, string | number>,
) => string;

// The only input is the authorized KAN-62 DTO. Do not read interview/bank endpoints here. A completed practice
// report is exportable with or without an overall (a withheld overall says why).
export function reportPrintDocument(
  report: Report,
  t: ReportTranslator,
  locale: string,
  font: string,
  number: (
    value: number,
    options?: Intl.NumberFormatOptions,
  ) => string = String,
  ts: ScoringTranslator = (key) => key,
) {
  if (
    report.type !== "practice" ||
    report.status !== "completed" ||
    !report.completed_at
  ) {
    throw new Error("Report is not exportable");
  }
  const day = report.completed_at.slice(0, 10);
  if (!/^\d{4}-\d{2}-\d{2}$/.test(day) || Number.isNaN(Date.parse(day)))
    throw new Error("Report is not exportable");
  const filename = `rolecue-practice-report-${day}.pdf`;
  const heading = (
    key: "overallScore" | "summary" | "breakdown" | "questions",
  ) => `<h2>${escape(t(key))}</h2>`;
  const paragraph = (text: string) => `<p>${escape(text)}</p>`;
  const score = (value: number) => formatScore(value, number);
  const turn = (item: NonNullable<Report["questions"]>[number]) =>
    paragraph(t("answer") + " " + (item.answer ?? t("noAnswer"))) +
    (item.feedback ? paragraph(t("feedback") + " " + item.feedback) : "");
  const scoring = report.scoring;
  const breakdown = report.breakdown ?? [];
  const formula = overallFormula(report, number);
  const overall =
    report.overall_score !== null
      ? paragraph(ts("outOf100", { score: score(report.overall_score) }))
      : paragraph(ts("withheld.title")) +
        (scoring?.withheld_reason
          ? paragraph(
              ts(`withheld.${scoring.withheld_reason}`, {
                metrics: uncoveredMetrics(breakdown)
                  .map((metric) => metric.name)
                  .join(", "),
              }),
            )
          : "");
  const outcome = scoring?.outcome
    ? paragraph(ts(`outcome.${scoring.outcome}`))
    : "";
  const metric = (item: (typeof breakdown)[number]) => {
    const value =
      item.score === null
        ? ts("noEvidence")
        : ts("outOf100", { score: score(item.score) });
    const weight =
      scoring?.method === "rubric"
        ? " — " +
          ((item.weight ?? 0) > 0
            ? ts("weight", {
                share: `${number(weightShare(item.weight ?? 0, scoring.weight_total), { maximumFractionDigits: 1 })}%`,
              })
            : ts("notWeighted"))
        : "";
    return `<li>${escape(item.name)}: ${escape(value + weight)}</li>`;
  };
  const grading = (questionNumber: number) => {
    const item = report.graded_questions?.find(
      (entry) => entry.question_number === questionNumber,
    );
    if (!item || item.status !== "graded") return "";
    return (
      `<div class="grading"><h4>${escape(ts("grading"))}: ${escape(ts("points", { points: number(item.earned), max: number(item.max) }))}` +
      (item.total !== null
        ? ` — ${escape(ts("questionTotal", { total: score(item.total) }))}`
        : "") +
      "</h4>" +
      // The step between the criteria and the session metric: the question's score per metric, as in the UI
      // (100 x earned / max over the question's criteria of that metric).
      (item.metrics.length > 0
        ? paragraph(
            ts("questionMetrics") +
              ": " +
              item.metrics
                .map((m) =>
                  ts("questionMetric", {
                    name: m.name,
                    earned: number(m.earned),
                    max: number(m.max),
                    score: score(m.score),
                  }),
                )
                .join("; "),
          )
        : "") +
      "<ul>" +
      item.criteria
        .map(
          (c) =>
            `<li>${escape((c.name || ts("criterionFallback", { metric: c.metric_name })) + " (" + c.metric_name + "): " + ts("points", { points: number(c.points), max: number(c.max_points) }))}` +
            (c.evidence
              ? paragraph(ts("evidence") + ": “" + c.evidence + "”")
              : "") +
            (c.feedback ? paragraph(ts("why") + ": " + c.feedback) : "") +
            "</li>",
        )
        .join("") +
      "</ul></div>"
    );
  };
  return `<!doctype html><html lang="${escape(locale)}"><head><meta charset="utf-8"><title>${escape(filename)}</title>
<style>
@font-face{font-family:Report;src:url("${font}") format("truetype");font-weight:100 900}
@page{size:A4;margin:18mm}
body{font-family:Report,sans-serif;font-size:11pt;line-height:1.6;color:#111}
h1{font-size:22pt}h2{font-size:15pt}h3{font-size:12pt}
h1,h2,h3,h4{break-after:avoid}p{white-space:pre-wrap;overflow-wrap:anywhere;orphans:3;widows:3}
li{margin-bottom:16px}.follow-up{border-left:2px solid #aaa;padding-left:14px}.grading{border-top:1px solid #ddd;margin-top:8px}
</style></head><body><h1>RoleCue — ${escape(t("type.practice"))}</h1>
${paragraph(day)}
${heading("overallScore")}${overall}${outcome}
${formula ? paragraph(ts("formulaRule")) + paragraph(formula) : ""}
${scoring?.method === "rubric" ? paragraph(ts("questionTotalNote")) : ""}
${report.summary ? heading("summary") + paragraph(report.summary) : ""}
${breakdown.length ? heading("breakdown") + "<ul>" + breakdown.map(metric).join("") + "</ul>" : ""}
${
  report.questions?.length
    ? heading("questions") +
      "<ol>" +
      groupByQuestion(report.questions)
        .map(
          ({ core, followUps }) =>
            `<li><h3>${escape(core.question)}</h3>${turn(core)}${followUps.map((item) => `<div class="follow-up"><h4>${escape(t("followUp"))}: ${escape(item.question)}</h4>${turn(item)}</div>`).join("")}${grading(core.question_number)}</li>`,
        )
        .join("") +
      "</ol>"
    : ""
}
${paragraph(ts("uncalibrated"))}
</body></html>`;
}

export async function printReport(
  report: Report,
  t: ReportTranslator,
  locale: string,
  target: Window,
  number: (value: number, options?: Intl.NumberFormatOptions) => string,
  ts: ScoringTranslator,
) {
  // Validate before loading the font, even when called directly.
  reportPrintDocument(report, t, locale, "", number, ts);
  const response = await fetch("/fonts/report.ttf");
  if (!response.ok) throw new Error("Report font unavailable");
  const bytes = new Uint8Array(await response.arrayBuffer());
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  const html = reportPrintDocument(
    report,
    t,
    locale,
    "data:font/ttf;base64," + btoa(binary),
    number,
    ts,
  );
  target.document.open();
  target.document.write(html);
  target.document.close();
  await target.document.fonts.load("11pt Report");
  await target.document.fonts.ready;
  if (target.closed) return;
  target.addEventListener("afterprint", () => target.close(), { once: true });
  target.focus();
  target.print();
}
