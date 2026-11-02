"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  evaluationKeys,
  getWeights,
} from "@/features/recruiter/evaluation/api/evaluation";
import type { JobPosting } from "@/features/recruiter/job-postings/schemas/job-posting-schema";
import { useFormatters } from "@/lib/i18n/use-formatters";

export function ReviewSubmissionDialog({
  postingId,
  posting,
  questionCount,
  rejected,
  pending,
  error,
  onCancel,
  onConfirm,
}: {
  postingId: string;
  posting: JobPosting | null;
  questionCount: number;
  rejected: boolean;
  pending: boolean;
  error: string | null;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const t = useTranslations("recruiter.questionBank.review");
  const tp = useTranslations("recruiter.postings");
  const tCommon = useTranslations("common");
  const fmt = useFormatters();
  const user = useSessionUser();
  const weights = useQuery({
    queryKey: evaluationKeys.weights(user.id, postingId),
    queryFn: ({ signal }) => getWeights(postingId, signal),
    refetchOnMount: "always",
    retry: false,
  });
  const weightValue = (percentage: number) =>
    fmt.number(percentage, { maximumFractionDigits: 2 });
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !pending) onCancel();
      }}
    >
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t("title")}</DialogTitle>
          <DialogDescription>{t("description")}</DialogDescription>
        </DialogHeader>
        {posting ? (
          <dl className="grid grid-cols-1 gap-4 rounded-xl border p-4 text-sm sm:grid-cols-2">
            <div className="sm:col-span-2">
              <dt className="text-muted-foreground">{t("posting")}</dt>
              <dd className="mt-1 font-medium">{posting.title}</dd>
            </div>
            <div>
              <dt className="text-muted-foreground">{t("seniority")}</dt>
              <dd className="mt-1">{tp(`level.${posting.seniority_level}`)}</dd>
            </div>
            <div>
              <dt className="text-muted-foreground">{t("bank")}</dt>
              <dd className="mt-1">
                {t("questionCount", { count: questionCount })}
              </dd>
            </div>
            <div>
              <dt className="text-muted-foreground">{t("interviewer")}</dt>
              <dd className="mt-1">
                {posting.required_avatar?.name ?? t("notSelected")}
              </dd>
            </div>
            <div>
              <dt className="text-muted-foreground">{t("voice")}</dt>
              <dd className="mt-1">
                {posting.required_voice?.name ?? t("notSelected")}
              </dd>
            </div>
            <div className="sm:col-span-2">
              <dt className="text-muted-foreground">{t("requirements")}</dt>
              <dd className="mt-1">
                {posting.requirements.map((item) => item.name).join(", ") ||
                  t("noRequirements")}
              </dd>
            </div>
          </dl>
        ) : (
          <p role="alert">{t("unavailable")}</p>
        )}
        <section
          className="space-y-3 text-sm"
          aria-label={t("evaluationLabel")}
        >
          <h3 className="font-medium">{t("evaluation")}</h3>
          {weights.isPending ? (
            <p role="status">{t("loadingCriteria")}</p>
          ) : weights.isError ? (
            <div role="alert">
              <p>{t("criteriaError")}</p>
              <Button variant="outline" onClick={() => void weights.refetch()}>
                {t("retryCriteria")}
              </Button>
            </div>
          ) : (
            <>
              <p className="text-muted-foreground">
                {weights.data.using_defaults
                  ? t("defaultCriteria")
                  : t("customCriteria")}
              </p>
              {!weights.data.editable && (
                <p role="alert">{t("alreadyReviewed")}</p>
              )}
              <dl className="divide-y">
                {weights.data.weights.map((weight) => (
                  <div
                    key={weight.metric_id}
                    className="flex justify-between gap-4 py-2"
                  >
                    <dt>{weight.name}</dt>
                    <dd className="tabular-nums">
                      {weights.data.total > 0
                        ? t("weightPercent", {
                            value: weightValue(weight.percentage),
                          })
                        : t("weightPoints", {
                            value: weightValue(weight.percentage),
                          })}
                    </dd>
                  </div>
                ))}
              </dl>
            </>
          )}
          <Button asChild variant="outline">
            <Link
              href={routes.recruiter.jobPostingEvaluation(postingId)}
              aria-disabled={pending || undefined}
              onClick={(event) => {
                if (pending) event.preventDefault();
              }}
            >
              {t("editCriteria")}
            </Link>
          </Button>
        </section>
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
        <DialogFooter>
          <Button variant="outline" onClick={onCancel} disabled={pending}>
            {tCommon("actions.cancel")}
          </Button>
          <Button
            disabled={
              pending ||
              !posting ||
              !weights.isSuccess ||
              !weights.data.editable ||
              questionCount === 0
            }
            onClick={onConfirm}
          >
            {pending ? t("submitting") : rejected ? t("resubmit") : t("submit")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
