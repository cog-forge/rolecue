"use client";

import { useEffect, useState, type ReactNode } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, CircleAlert } from "lucide-react";
import { gooeyToast } from "goey-toast";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { MetricCoverage } from "@/components/feedback/metric-coverage";
import { Skeleton } from "@/components/ui/skeleton";
import { coverageBlocks } from "@/lib/api/coverage-schema";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  approvePosting,
  getPosting,
  moderationKeys,
  rejectPosting,
} from "../api/moderation";
import type { ReviewPosting } from "../schemas/moderation-schema";
import { apiError, isAccessDenied } from "../utils/api-error";
import { DecisionDialog, type Decision } from "./decision-dialog";
import { JdViewer } from "./jd-viewer";
import { PostingStatus } from "./posting-status";
import type { Formatters } from "@/lib/i18n/formatters";
import { useApiErrorMessage } from "@/lib/i18n/use-api-error-message";
import { useFormatters } from "@/lib/i18n/use-formatters";

type DetailsTranslator = ReturnType<
  typeof useTranslations<"admin.moderation.review.details">
>;

// Units and currency aren't defined in the schema yet, so numbers are shown as stored.
function salary(posting: ReviewPosting, fmt: Formatters, t: DetailsTranslator) {
  const { salary_min: min, salary_max: max } = posting;
  if (min === null && max === null) return null;
  if (min !== null && max !== null)
    return t("salaryRange", { min: fmt.number(min), max: fmt.number(max) });
  return min !== null
    ? t("salaryFrom", { amount: fmt.number(min) })
    : t("salaryUpTo", { amount: fmt.number(max!) });
}

export function PostingReview({
  id,
  backQuery,
}: {
  id: string;
  backQuery: string;
}) {
  const t = useTranslations("admin.moderation.review");
  const tCommon = useTranslations("common");
  const errorMessage = useApiErrorMessage();
  const user = useSessionUser();
  const cache = useQueryClient();
  const key = moderationKeys.detail(user.id, id);
  const [decision, setDecision] = useState<Decision | null>(null);
  // Kept as the error, not text, so the notice follows the language switcher.
  const [notice, setNotice] = useState<{ error: unknown } | null>(null);
  const query = useQuery({
    queryKey: key,
    queryFn: ({ signal }) => getPosting(id, signal),
    refetchOnMount: "always",
    retry: false,
  });
  const decide = useMutation({
    mutationFn: (input: { decision: Decision; reason?: string }) =>
      input.decision === "approve"
        ? approvePosting(id)
        : rejectPosting(id, input.reason ?? ""),
    retry: false,
    onSuccess: async (posting, input) => {
      await cache.cancelQueries({ queryKey: key });
      cache.setQueryData(key, posting);
      void cache.invalidateQueries({ queryKey: moderationKeys.lists(user.id) });
      setDecision(null);
      setNotice(null);
      gooeyToast.success(
        input.decision === "approve"
          ? t("toast.approvedTitle")
          : t("toast.rejectedTitle"),
        {
          description:
            input.decision === "approve"
              ? t("toast.approvedDescription")
              : t("toast.rejectedDescription"),
        },
      );
    },
    onError: (error) => {
      const { status } = apiError(error);
      if (status === 409) {
        // Someone else decided first, or the bank is empty: show why and reload the truth.
        setDecision(null);
        setNotice({ error });
        void cache.invalidateQueries({ queryKey: key });
        void cache.invalidateQueries({
          queryKey: moderationKeys.lists(user.id),
        });
      }
    },
  });
  const denied = isAccessDenied(query.error) || isAccessDenied(decide.error);
  useEffect(() => {
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);

  const backHref = `${routes.admin.jobPostings}${backQuery ? `?${backQuery}` : ""}`;
  const dialogError =
    decision && decide.isError && apiError(decide.error).status !== 409
      ? errorMessage(decide.error, t("saveFailed"))
      : null;

  return (
    <section className="mx-auto w-full max-w-6xl space-y-6">
      <Link
        href={backHref}
        className="inline-flex min-h-11 items-center gap-2 rounded-md text-sm font-medium text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        <ArrowLeft className="size-4" aria-hidden />
        {t("back")}
      </Link>

      {query.isPending ? (
        <div role="status" className="space-y-4">
          <Skeleton className="h-10 w-2/3" />
          <Skeleton className="h-48 rounded-xl" />
          <Skeleton className="h-96 rounded-xl" />
          <span className="sr-only">{t("loading")}</span>
        </div>
      ) : denied ? (
        <div role="alert" className="rounded-xl border p-6">
          <h1 className="text-xl font-semibold">{t("denied.title")}</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {t("denied.description")}
          </p>
        </div>
      ) : query.isError ? (
        <div role="alert" className="rounded-xl border p-6">
          <h1 className="text-xl font-semibold">
            {apiError(query.error).status === 404
              ? t("notFound.title")
              : t("loadFailed.title")}
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {apiError(query.error).status === 404
              ? t("notFound.description")
              : tCommon("messages.tryAgainLater")}
          </p>
          {apiError(query.error).status !== 404 && (
            <Button
              className="mt-4 h-11"
              variant="outline"
              disabled={query.isFetching}
              onClick={() => void query.refetch()}
            >
              {tCommon("actions.tryAgain")}
            </Button>
          )}
        </div>
      ) : (
        <>
          <header className="space-y-2">
            <h1 className="text-2xl font-semibold tracking-tight text-balance [overflow-wrap:anywhere] sm:text-[1.75rem] sm:leading-tight">
              {query.data.title}
            </h1>
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 text-sm text-muted-foreground">
              {/* A pending posting is in the queue only once submitted; before that it is not
                  "Pending review" yet. */}
              {query.data.status === "pending" &&
              !query.data.review_submitted ? (
                <span>{t("preparing")}</span>
              ) : (
                <PostingStatus status={query.data.status} />
              )}
            </div>
          </header>

          {notice && (
            <p
              role="alert"
              className="flex items-start gap-2 rounded-lg border border-destructive/30 p-3 text-sm"
            >
              <CircleAlert
                className="mt-0.5 size-4 shrink-0 text-destructive"
                aria-hidden
              />
              {errorMessage(notice.error, t("conflict"))}
            </p>
          )}

          <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_20rem]">
            <div className="space-y-6">
              <PostingDossier posting={query.data} />
              <MetricCoverage coverage={query.data.coverage} audience="admin" />
              <Card>
                <CardContent className="pt-6">
                  <JdViewer
                    contentPath={query.data.content_path}
                    documentUrl={query.data.document_url}
                    rawText={query.data.raw_text}
                  />
                </CardContent>
              </Card>
            </div>
            <div className="space-y-6">
              <DecisionPanel
                posting={query.data}
                onDecide={(next) => {
                  decide.reset();
                  setDecision(next);
                }}
              />
              <DetailsCard posting={query.data} />
            </div>
          </div>

          <DecisionDialog
            decision={decision}
            title={query.data.title}
            pending={decide.isPending}
            error={dialogError}
            onCancel={() => setDecision(null)}
            onApprove={() => decide.mutate({ decision: "approve" })}
            onReject={(reason) => decide.mutate({ decision: "reject", reason })}
          />
        </>
      )}
    </section>
  );
}

function DecisionPanel({
  posting,
  onDecide,
}: {
  posting: ReviewPosting;
  onDecide: (decision: Decision) => void;
}) {
  const t = useTranslations("admin.moderation.review.decision");
  const tCoverage = useTranslations("common.coverage");
  const fmt = useFormatters();
  const count = posting.core_question_count;
  // A posting still being prepared cannot be decided; approval also needs questions, and a bank that
  // measures every metric the posting is scored on (the server refuses it otherwise, KAN-377).
  const notSubmitted = !posting.review_submitted;
  const uncovered = coverageBlocks(posting.coverage);
  const approveBlocked = notSubmitted || count === 0 || uncovered;
  return (
    <section
      aria-labelledby="decision-heading"
      className="space-y-4 rounded-xl border p-5"
    >
      <h2 id="decision-heading" className="text-base font-semibold">
        {t("heading")}
      </h2>
      <div className="space-y-1">
        <p className="text-sm text-muted-foreground">{t("questionBank")}</p>
        <p className="text-sm">
          <span className="text-2xl font-semibold tabular-nums">
            {fmt.number(count)}
          </span>{" "}
          {t("coreQuestions", { count })}
        </p>
      </div>
      {posting.status !== "pending" ? (
        <p className="text-sm text-muted-foreground">
          {t("decided")}
          {posting.status === "rejected" && posting.reject_reason && (
            <>
              {" "}
              {t("reasonGiven")}{" "}
              <span className="text-foreground [overflow-wrap:anywhere]">
                “{posting.reject_reason}”
              </span>
            </>
          )}
        </p>
      ) : (
        <>
          {approveBlocked && (
            <p id="approve-blocked" className="text-sm text-muted-foreground">
              {notSubmitted
                ? t("notSubmitted")
                : count === 0
                  ? t("noQuestions")
                  : tCoverage("blockedAdmin")}
            </p>
          )}
          <div className="flex flex-col gap-2">
            <Button
              className="h-10 rounded-full"
              disabled={approveBlocked}
              aria-describedby={approveBlocked ? "approve-blocked" : undefined}
              onClick={() => onDecide("approve")}
            >
              {t("approve")}
            </Button>
            <Button
              variant="outline"
              className="h-10 rounded-full"
              disabled={notSubmitted}
              aria-describedby={notSubmitted ? "approve-blocked" : undefined}
              onClick={() => onDecide("reject")}
            >
              {t("reject")}
            </Button>
          </div>
        </>
      )}
    </section>
  );
}

function DetailsCard({ posting }: { posting: ReviewPosting }) {
  const t = useTranslations("admin.moderation.review.details");
  const tCommon = useTranslations("common");
  const fmt = useFormatters();
  const rows: [string, ReactNode][] = [
    [
      t("recruiter"),
      <>
        {posting.recruiter.name}
        <span className="block text-xs text-muted-foreground [overflow-wrap:anywhere]">
          {posting.recruiter.email}
        </span>
      </>,
    ],
    [
      t("created"),
      <time key="t" dateTime={posting.created_at}>
        {t("createdAt", {
          date: fmt.date(posting.created_at, "calendarDayTime"),
        })}
      </time>,
    ],
    [t("company"), posting.company_name ?? null],
    [t("seniority"), posting.seniority_level ?? null],
    [t("location"), posting.location],
    [t("employmentType"), posting.employment_type],
    [t("salary"), salary(posting, fmt, t)],
    [
      t("interviewSlots"),
      posting.interview_slot === null
        ? null
        : fmt.number(posting.interview_slot),
    ],
    [
      t("interviewDuration"),
      posting.interview_duration === null
        ? null
        : t("minutes", { minutes: posting.interview_duration }),
    ],
    [t("avatar"), posting.required_avatar?.name ?? null],
    [t("voice"), posting.required_voice?.name ?? null],
  ];
  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2>{t("title")}</h2>
        </CardTitle>
      </CardHeader>
      <CardContent>
        <dl className="divide-y text-sm">
          {rows.map(([label, value]) => (
            <div
              key={label}
              className="grid grid-cols-[minmax(6.5rem,8.5rem)_minmax(0,1fr)] gap-3 py-2.5"
            >
              <dt className="min-w-0 text-muted-foreground [overflow-wrap:anywhere]">
                {label}
              </dt>
              <dd className="min-w-0 [overflow-wrap:anywhere]">
                {value ?? (
                  <span className="text-muted-foreground">
                    {tCommon("labels.notSet")}
                  </span>
                )}
              </dd>
            </div>
          ))}
        </dl>
      </CardContent>
    </Card>
  );
}

function PostingDossier({ posting }: { posting: ReviewPosting }) {
  const t = useTranslations("admin.moderation.review.dossier");
  const fmt = useFormatters();
  const evaluation = posting.evaluation;
  return (
    <>
      <section
        aria-labelledby="confirmed-requirements-heading"
        className="space-y-4 rounded-xl border p-5"
      >
        <h2
          id="confirmed-requirements-heading"
          className="text-base font-semibold"
        >
          {t("requirementsTitle")}
        </h2>
        <p className="text-sm text-muted-foreground">
          {t("requirementsDescription")}
        </p>
        {posting.requirements && posting.requirements.length > 0 ? (
          <ul className="divide-y text-sm">
            {posting.requirements.map((requirement, index) => (
              <li
                key={`${requirement.target}-${requirement.name}-${index}`}
                className="flex flex-wrap justify-between gap-2 py-2"
              >
                <span className="font-medium">{requirement.name}</span>
                <span className="text-muted-foreground">
                  <span className="capitalize">
                    {requirement.target.replaceAll("_", " ")}
                  </span>
                  {" · "}
                  {t(`requirementType.${requirement.type}`)}
                </span>
              </li>
            ))}
          </ul>
        ) : (
          <p className="text-sm text-muted-foreground">
            {t("requirementsEmpty")}
          </p>
        )}
        {posting.refinement_note && (
          <div className="space-y-1 text-sm">
            <h3 className="font-medium">{t("guidance")}</h3>
            <p className="whitespace-pre-wrap text-muted-foreground">
              {posting.refinement_note}
            </p>
          </div>
        )}
      </section>
      <section
        aria-labelledby="review-evaluation-heading"
        className="space-y-4 rounded-xl border p-5"
      >
        <h2 id="review-evaluation-heading" className="text-base font-semibold">
          {t("evaluationTitle")}
        </h2>
        {evaluation ? (
          <>
            <p className="text-sm text-muted-foreground">
              {evaluation.using_defaults
                ? t("evaluationDefaults")
                : t("evaluationCustom")}
            </p>
            <dl className="divide-y text-sm">
              {evaluation.weights.map((weight) => (
                <div
                  key={weight.metric_id}
                  className="flex justify-between gap-4 py-2"
                >
                  <dt>{weight.name}</dt>
                  <dd className="tabular-nums">
                    {t(evaluation.total > 0 ? "weightPercent" : "weightPlain", {
                      value: fmt.number(weight.percentage, {
                        maximumFractionDigits: 2,
                      }),
                    })}
                  </dd>
                </div>
              ))}
            </dl>
          </>
        ) : (
          <p className="text-sm text-muted-foreground">
            {t("evaluationUnavailable")}
          </p>
        )}
      </section>
    </>
  );
}
