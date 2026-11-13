"use client";
import { ErrorState } from "@/components/ui/error-state";

import { useEffect, useRef, useState, type ReactNode } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { gooeyToast } from "goey-toast";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { routes } from "@/config/routes";
import { cn } from "@/lib/utils";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  capacityKeys,
  getCapacity,
} from "@/features/recruiter/capacity/api/capacity";
import { CapacityPanel } from "@/features/recruiter/capacity/components/capacity-panel";
import { getPosting, jobPostingKeys, updatePosting } from "../api/job-postings";
import {
  postingLocks,
  postingPatch,
  postingValues,
  type JobPosting,
} from "../schemas/job-posting-schema";
import {
  generateBank,
  questionBankKeys,
} from "@/features/recruiter/question-bank/api/question-bank";
import { apiError, isAccessDenied } from "../utils/api-error";
import { nextStep, primaryAction, statusLine } from "../utils/posting-guidance";
import { PostingApplications } from "./posting-applications";
import { JdDocumentLink } from "./jd-document-link";
import { FormSection, PostingForm } from "./posting-form";
import { Notice } from "./notice";
import { PageHeader, pillClass } from "@/components/layout/page-header";
import { PostingProgress } from "./posting-progress";
import { PostingStatus } from "./posting-status";
import { PostingTabs } from "./posting-tabs";
import { useFormatters } from "@/lib/i18n/use-formatters";
import { useApiErrorMessage } from "@/lib/i18n/use-api-error-message";

// Codes after which the cached posting is stale: its status or question bank changed elsewhere.
// Refetching shows the server's truth and the matching locks.
const staleCodes = new Set([
  "QUESTION_BANK_LOCKED",
  "JOB_POSTING_NOT_EDITABLE",
  "JOB_POSTING_TEXT_LOCKED",
  "JOB_POSTING_REQUIREMENTS_LOCKED",
]);

export function JobPostingDetail({ postingId }: { postingId: string }) {
  const t = useTranslations("recruiter.postings.detail");
  const tCommon = useTranslations("common");
  const tp = useTranslations("recruiter.postings");
  const tg = useTranslations("recruiter.postings.guidance");
  const errorMessage = useApiErrorMessage();
  const fmt = useFormatters();
  const user = useSessionUser();
  const cache = useQueryClient();
  const router = useRouter();
  const key = jobPostingKeys.detail(user.id, postingId);
  const [notice, setNotice] = useState<string | null>(null);
  // Remounts the form with the saved values after each successful save.
  const [version, setVersion] = useState(0);
  // Validation is async, so two quick clicks can both reach onSubmit before isPending flips.
  const saving = useRef(false);

  const query = useQuery({
    queryKey: key,
    queryFn: ({ signal }) => getPosting(postingId, signal),
    refetchOnMount: "always",
    retry: false,
  });
  const denied = isAccessDenied(query.error);
  useEffect(() => {
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);

  // The same request the capacity panel makes, so the header's slot figures cost nothing extra.
  const capacity = useQuery({
    queryKey: capacityKeys.detail(user.id, postingId),
    queryFn: ({ signal }) => getCapacity(postingId, signal),
    refetchOnMount: "always",
    retry: false,
  });

  const applySaved = (saved: JobPosting) => {
    cache.setQueryData(key, saved);
    void cache.invalidateQueries({ queryKey: jobPostingKeys.lists(user.id) });
    setVersion((value) => value + 1);
  };
  const onStale = (error: unknown) => {
    if (isAccessDenied(error))
      void cache.invalidateQueries({ queryKey: ["auth", "me"] });
    const { code } = apiError(error);
    if (!code || !staleCodes.has(code)) return;
    setNotice(
      t("refreshed", { reason: errorMessage(error, t("changedReason")) }),
    );
    // Remount the form on the fresh posting: the edited values were made against stale locks and
    // would be sent (and refused) again on every save.
    void cache
      .refetchQueries({ queryKey: key })
      .then(() => setVersion((value) => value + 1));
  };

  const save = useMutation({
    // Saving and generating are separate requests: the saved posting is cached at once, so a failed
    // generation keeps the saved form and a retry only generates.
    mutationFn: async ({
      patch,
      current,
      generate,
    }: {
      patch: Parameters<typeof updatePosting>[1];
      current: JobPosting;
      generate: boolean;
    }) => {
      let saved = current;
      if (Object.keys(patch).length) {
        saved = await updatePosting(postingId, patch);
        cache.setQueryData(key, saved);
      }
      if (generate) await generateBank(postingId);
      return { saved, generated: generate };
    },
    retry: false,
    onSuccess: ({ saved, generated }) => {
      setNotice(null);
      if (!generated) {
        applySaved(saved);
        gooeyToast.success(t("saved"));
        return;
      }
      void cache.invalidateQueries({ queryKey: key });
      void cache.invalidateQueries({ queryKey: jobPostingKeys.lists(user.id) });
      // Generation replaces the bank; discard any version warmed by the tabs.
      cache.removeQueries({
        queryKey: questionBankKeys.detail(user.id, postingId),
        exact: true,
      });
      gooeyToast.success(tp("toast.bankGenerated"), {
        description: tp("toast.bankGeneratedHint"),
      });
      router.push(routes.recruiter.jobPostingQuestions(postingId));
    },
    onError: onStale,
  });

  const back = {
    href: routes.recruiter.jobPostings,
    label: tp("backToList"),
  };

  if (query.isPending)
    return (
      <section className="mx-auto w-full max-w-5xl space-y-8">
        <PageHeader
          back={back}
          title={<Skeleton className="h-8 w-2/3 rounded-md" />}
        />
        <div role="status" className="space-y-3">
          <Skeleton className="h-12 rounded-md" />
          <Skeleton className="h-64 rounded-xl" />
          <span className="sr-only">{t("loading")}</span>
        </div>
      </section>
    );
  if (query.isError) {
    // 400 is a malformed ID in the URL; retrying cannot help, just like a missing posting.
    const notFound = [400, 404].includes(apiError(query.error).status);
    return (
      <section className="mx-auto w-full max-w-5xl space-y-8">
        <PageHeader back={back} title={t("fallbackTitle")} />
        <ErrorState
          title={
            denied
              ? t("error.deniedTitle")
              : notFound
                ? t("error.notFoundTitle")
                : t("error.failedTitle")
          }
          description={
            denied
              ? t("error.deniedBody")
              : notFound
                ? t("error.notFoundBody")
                : tCommon("messages.tryAgainLater")
          }
        >
          {!denied && !notFound && (
            <Button
              className={cn("mt-4", pillClass)}
              variant="outline"
              disabled={query.isFetching}
              onClick={() => void query.refetch()}
            >
              {tCommon("actions.tryAgain")}
            </Button>
          )}
        </ErrorState>
      </section>
    );
  }

  const posting = query.data;
  const locks = postingLocks(posting);
  const hasBank = posting.core_question_count > 0;
  const action = primaryAction(posting, tg);
  const line = statusLine(posting, fmt, tg, capacity.data);
  const generating = save.isPending && save.variables?.generate === true;
  const saveError =
    save.isError && !staleCodes.has(apiError(save.error).code ?? "")
      ? isAccessDenied(save.error)
        ? t("saveDenied")
        : t("saveFailed", {
            reason: errorMessage(save.error, t("saveFailedReason")),
          })
      : null;

  return (
    <section className="mx-auto w-full max-w-5xl space-y-8">
      <PageHeader
        back={back}
        title={posting.title}
        meta={
          <>
            <PostingStatus
              status={posting.status}
              questionBankReady={hasBank}
              reviewSubmitted={posting.review_submitted}
              interviewSlot={posting.interview_slot}
            />
            <span>
              {t.rich("created", {
                date: fmt.date(posting.created_at, "calendarDay"),
                time: (chunks) => (
                  <time
                    dateTime={posting.created_at}
                    title={posting.created_at}
                  >
                    {chunks}
                  </time>
                ),
              })}
            </span>
            <span>
              {t("slots")}{" "}
              <span className="font-medium text-foreground tabular-nums">
                {posting.interview_slot ?? t("notFunded")}
              </span>
            </span>
            {line && (
              <span className="font-medium text-foreground">{line}</span>
            )}
          </>
        }
        description={nextStep(posting, tg)}
        actions={
          action && (
            <Button asChild className={pillClass}>
              <Link href={action.href}>{action.label}</Link>
            </Button>
          )
        }
      />
      <PostingTabs
        postingId={posting.id}
        active="overview"
        status={posting.status}
      />
      <PostingProgress posting={posting} generating={generating} />

      {notice && (
        <Notice role="status" tone="info">
          <p>{notice}</p>
        </Notice>
      )}

      {posting.status === "rejected" && (
        <Notice tone="danger" title={t("rejected.title")}>
          {posting.reject_reason && <p>“{posting.reject_reason}”</p>}
          <p className="text-muted-foreground">
            {t.rich("rejected.body", {
              bank: (chunks) => (
                <BankLink postingId={posting.id}>{chunks}</BankLink>
              ),
            })}
          </p>
        </Notice>
      )}

      <CapacityPanel postingId={posting.id} />

      {locks.editable ? (
        <PostingForm
          key={version}
          mode="edit"
          posting={posting}
          defaultValues={postingValues(posting)}
          locks={locks}
          submitting={save.isPending}
          submitError={saveError}
          onSubmit={(values, requirementsChanged) => {
            const patch = postingPatch(values, posting);
            if (
              saving.current ||
              (Object.keys(patch).length === 0 && !requirementsChanged)
            )
              return;
            saving.current = true;
            save.mutate(
              { patch, current: posting, generate: requirementsChanged },
              {
                onSettled: () => {
                  saving.current = false;
                },
              },
            );
          }}
        />
      ) : (
        <ReadOnlyPosting posting={posting} />
      )}

      <PostingApplications posting={posting} />
    </section>
  );
}

function BankLink({
  postingId,
  children,
}: {
  postingId: string;
  children: ReactNode;
}) {
  return (
    <Link
      href={routes.recruiter.jobPostingQuestions(postingId)}
      className="font-medium underline underline-offset-4"
    >
      {children}
    </Link>
  );
}

function ReadOnlyPosting({ posting }: { posting: JobPosting }) {
  const t = useTranslations("recruiter.postings.detail.readOnly");
  const tp = useTranslations("recruiter.postings");
  const tCommon = useTranslations("common");
  const details: [string, string][] = [
    [t("seniority"), tp(`level.${posting.seniority_level}`)],
    [t("refinementNote"), posting.refinement_note || t("none")],
    [t("location"), posting.location ?? tCommon("labels.notSet")],
    [t("employmentType"), posting.employment_type ?? tCommon("labels.notSet")],
    [
      t("salary"),
      posting.salary_min === null && posting.salary_max === null
        ? tCommon("labels.notSet")
        : `${posting.salary_min ?? "…"} – ${posting.salary_max ?? "…"}`,
    ],
    [
      t("interviewLength"),
      posting.interview_duration === null
        ? tCommon("labels.notSet")
        : t("minutes", { count: posting.interview_duration }),
    ],
    [
      t("interviewerModel"),
      posting.required_avatar?.name ?? tCommon("labels.notSet"),
    ],
    [
      t("interviewerVoice"),
      posting.required_voice?.name ?? tCommon("labels.notSet"),
    ],
    [
      t("candidatesAllowed"),
      posting.interview_slot === null
        ? tCommon("labels.notSet")
        : String(posting.interview_slot),
    ],
  ];
  return (
    <div>
      <FormSection
        title={t("details.title")}
        description={t("details.description")}
      >
        <dl className="grid gap-x-8 gap-y-4 text-sm sm:grid-cols-2">
          {details.map(([label, value]) => (
            <div key={label} className="min-w-0 space-y-0.5">
              <dt className="text-muted-foreground">{label}</dt>
              <dd className="font-medium [overflow-wrap:anywhere]">{value}</dd>
            </div>
          ))}
        </dl>
      </FormSection>
      <FormSection
        title={t("requirements.title")}
        description={t("requirements.description")}
      >
        <ul className="divide-y rounded-lg border text-sm">
          {posting.requirements.map((row) => (
            <li
              key={`${row.name}-${row.target}`}
              className="flex flex-wrap justify-between gap-2 px-3 py-2.5"
            >
              <span className="font-medium [overflow-wrap:anywhere]">
                {row.name}
              </span>
              <span className="text-muted-foreground">
                {tp("requirementMeta", {
                  kind: tp(`target.${row.target}`),
                  priority: tp(`priority.${row.type}`),
                })}
              </span>
            </li>
          ))}
        </ul>
      </FormSection>
      <FormSection
        title={t("jobDescription.title")}
        description={t("jobDescription.description")}
        contentClassName="space-y-3"
      >
        {posting.has_document && <JdDocumentLink postingId={posting.id} />}
        <div className="max-h-96 overflow-y-auto rounded-lg border bg-muted/50 p-4 text-sm leading-relaxed whitespace-pre-wrap [overflow-wrap:anywhere]">
          {posting.raw_text}
        </div>
      </FormSection>
    </div>
  );
}
