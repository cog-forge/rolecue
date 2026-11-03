import type { Report } from "../schemas/report-schema";
import { groupByQuestion } from "./question-groups";
import type { ReportTranslator } from "./report-text";

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

// The only input is the authorized KAN-62 DTO. Do not read interview/bank endpoints here.
export function reportPrintDocument(
  report: Report,
  t: ReportTranslator,
  locale: string,
  font: string,
  number: (value: number) => string = String,
) {
  if (
    report.type !== "practice" ||
    report.status !== "completed" ||
    !report.completed_at ||
    report.overall_score === null
  ) {
    throw new Error("Report is not exportable");
  }
  const filename = `rolecue-practice-report-${report.completed_at.slice(0, 10)}.pdf`;
  const heading = (
    key: "overallScore" | "summary" | "breakdown" | "questions",
  ) => `<h2>${escape(t(key))}</h2>`;
  const paragraph = (text: string) => `<p>${escape(text)}</p>`;
  const turn = (item: NonNullable<Report["questions"]>[number]) =>
    paragraph(t("answer") + " " + (item.answer ?? t("noAnswer"))) +
    (item.feedback ? paragraph(t("feedback") + " " + item.feedback) : "");
  return `<!doctype html><html lang="${escape(locale)}"><head><meta charset="utf-8"><title>${filename}</title>
<style>
@font-face{font-family:Report;src:url("${font}") format("truetype");font-weight:100 900}
@page{size:A4;margin:18mm}
body{font-family:Report,sans-serif;font-size:11pt;line-height:1.6;color:#111}
h1{font-size:22pt}h2{font-size:15pt}h3{font-size:12pt}
h1,h2,h3,h4{break-after:avoid}p{white-space:pre-wrap;overflow-wrap:anywhere;orphans:3;widows:3}
li{margin-bottom:16px}.follow-up{border-left:2px solid #aaa;padding-left:14px}
</style></head><body><h1>RoleCue — ${escape(t("type.practice"))}</h1>
${paragraph(report.completed_at.slice(0, 10))}
${heading("overallScore")}${paragraph(t("score", { score: number(report.overall_score) }))}
${report.summary ? heading("summary") + paragraph(report.summary) : ""}
${report.breakdown?.length ? heading("breakdown") + "<ul>" + report.breakdown.map((metric) => `<li>${escape(metric.name)}: ${escape(t("score", { score: number(metric.score) }))}</li>`).join("") + "</ul>" : ""}
${
  report.questions?.length
    ? heading("questions") +
      "<ol>" +
      groupByQuestion(report.questions)
        .map(
          ({ core, followUps }) =>
            `<li><h3>${escape(core.question)}</h3>${turn(core)}${followUps.map((item) => `<div class="follow-up"><h4>${escape(t("followUp"))}: ${escape(item.question)}</h4>${turn(item)}</div>`).join("")}</li>`,
        )
        .join("") +
      "</ol>"
    : ""
}
</body></html>`;
}

export async function printReport(
  report: Report,
  t: ReportTranslator,
  locale: string,
  target: Window,
  number: (value: number) => string,
) {
  // Validate before loading the font, even when called directly.
  reportPrintDocument(report, t, locale, "", number);
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
