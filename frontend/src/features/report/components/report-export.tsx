"use client";

import { useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { useFormatters } from "@/lib/i18n/use-formatters";
import { Button } from "@/components/ui/button";
import { getInterviewReport } from "../api/report";
import type { Report } from "../schemas/report-schema";

export function ReportExport({ report }: { report: Report }) {
  const t = useTranslations("candidate.report");
  const ts = useTranslations("candidate.report.scoring");
  const locale = useLocale();
  const fmt = useFormatters();
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  if (
    report.type !== "practice" ||
    report.status !== "completed" ||
    !report.completed_at
  )
    return null;

  async function exportPdf() {
    setFailed(false);
    // Open during the gesture, before async work, so popup blockers permit the print preview.
    const target = window.open("", "_blank", "popup");
    if (!target) {
      setFailed(true);
      return;
    }
    target.opener = null;
    setBusy(true);
    try {
      // Re-authorize ownership and current state, including when the page has cached data.
      const current = await getInterviewReport(report.interview_id);
      const { printReport } = await import("../utils/report-pdf");
      await printReport(current, t, locale, target, fmt.number, (key, values) =>
        // The keys come from the report's own scoring catalog (candidate.report.scoring).
        ts(key as Parameters<typeof ts>[0], values),
      );
    } catch {
      target.close();
      setFailed(true);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-2">
      <Button disabled={busy} onClick={() => void exportPdf()}>
        {t(busy ? "exporting" : "exportPdf")}
      </Button>
      <p className="text-sm text-muted-foreground">{t("exportHint")}</p>
      {failed ? (
        <p role="alert" className="text-sm text-destructive">
          {t("exportError")}
        </p>
      ) : null}
    </div>
  );
}
