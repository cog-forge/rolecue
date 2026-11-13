"use client";
import { ErrorState } from "@/components/ui/error-state";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { Accordion as AccordionPrimitive } from "radix-ui";
import { ChevronDown } from "lucide-react";
import {
  Accordion,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { getRubric, getRubricMetrics, rubricKeys } from "../api/rubrics";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { useApiErrorMessage } from "@/lib/i18n/use-api-error-message";

// Shared read-only criteria presentation for bank review, Admin views and reports.
export function RubricCriteria({ rubricId }: { rubricId: string }) {
  const [open, setOpen] = useState(false);
  const t = useTranslations("common.rubricReview");
  const user = useSessionUser();
  const errorMessage = useApiErrorMessage();
  const rubric = useQuery({
    queryKey: [...rubricKeys.all(user.id), "detail", rubricId],
    queryFn: ({ signal }) => getRubric(rubricId, signal),
    enabled: open,
    retry: false,
  });
  const metrics = useQuery({
    queryKey: [...rubricKeys.all(user.id), "metrics"],
    queryFn: ({ signal }) => getRubricMetrics(signal),
    enabled: open,
    retry: false,
  });
  const error = rubric.error ?? metrics.error;
  return (
    <Accordion
      type="single"
      collapsible
      value={open ? "criteria" : ""}
      onValueChange={(value) => setOpen(value === "criteria")}
    >
      <AccordionItem
        value="criteria"
        className="t-acc border-none"
        data-open={String(open)}
      >
        <AccordionTrigger
          showIndicator={false}
          className="t-acc-head min-h-11 items-center justify-start gap-2 text-xs font-medium text-rolecue-brand"
        >
          {t("viewCriteria")}
          <ChevronDown className="t-acc-chevron size-4" aria-hidden />
        </AccordionTrigger>
        <AccordionPrimitive.Content
          forceMount
          className="t-acc-panel"
          inert={!open}
          aria-hidden={!open}
        >
          <div className="t-acc-panel-inner">
            <div className="space-y-3 pb-4">
              {(rubric.isPending || metrics.isPending) && (
                <p role="status" className="text-sm text-muted-foreground">
                  {t("loading")}
                </p>
              )}
              {error && (
                <ErrorState
                  description={errorMessage(error, t("failed"))}
                  onRetry={() => {
                    void rubric.refetch();
                    void metrics.refetch();
                  }}
                  retryLabel={t("retry")}
                ></ErrorState>
              )}
              {!error &&
                rubric.data?.criteria?.map((criterion, index) => (
                  <div
                    key={criterion.id ?? index}
                    className="grid gap-2 border-l-2 border-rolecue-brand/40 pl-3 sm:grid-cols-[minmax(0,1fr)_auto]"
                  >
                    <div className="min-w-0 space-y-1">
                      <p className="text-sm font-medium wrap-anywhere">
                        {criterion.name}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        {metrics.data?.find(
                          (metric) => metric.id === criterion.metric_id,
                        )?.name ?? t("metricUnavailable")}
                      </p>
                      <p className="text-sm whitespace-pre-line wrap-anywhere">
                        {criterion.scoring_guide}
                      </p>
                    </div>
                    <span className="h-fit rounded-full bg-rolecue-brand/10 px-2.5 py-1 text-xs font-semibold text-rolecue-brand tabular-nums">
                      {t("points", { count: criterion.max_points })}
                    </span>
                  </div>
                ))}
            </div>
          </div>
        </AccordionPrimitive.Content>
      </AccordionItem>
    </Accordion>
  );
}
