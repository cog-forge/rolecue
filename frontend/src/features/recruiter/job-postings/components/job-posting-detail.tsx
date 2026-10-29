"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
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
import { generateBank } from "@/features/recruiter/question-bank/api/question-bank";
import { apiError, isAccessDenied } from "../utils/api-error";
import { primaryAction, statusLine } from "../utils/posting-guidance";
import { PostingApplications } from "./posting-applications";
import { JdDocumentLink } from "./jd-document-link";
import {
  FormSection,
  levelText,
  PostingForm,
  targetText,
} from "./posting-form";
import { Notice } from "./notice";
import { PageHeader, pillClass } from "@/components/layout/page-header";
import { PostingProgress } from "./posting-progress";
import { PostingStatus } from "./posting-status";
import { PostingTabs } from "./posting-tabs";
import { useFormatters } from "@/lib/i18n/use-formatters";

// Codes after which the cached posting is stale: its status or question bank changed elsewhere.
// Refetching shows the server's truth and the matching locks.
const staleCodes = new Set([
  "QUESTION_BANK_LOCKED",
  "JOB_POSTING_NOT_EDITABLE",
  "JOB_POSTING_TEXT_LOCKED",
  "JOB_POSTING_REQUIREMENTS_LOCKED",
]);

export function JobPostingDetail({ postingId }: { postingId: string }) {
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
    const { code, message } = apiError(error);
    if (!code || !staleCodes.has(code)) return;
    setNotice(`${message ?? "This posting changed."} We refreshed it for you.`);
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
        gooeyToast.success("Changes saved");
        return;
      }
      void cache.invalidateQueries({ queryKey: key });
      void cache.invalidateQueries({ queryKey: jobPostingKeys.lists(user.id) });
      gooeyToast.success("Question bank generated", {
        description: "Review your questions before submitting for approval.",
      });
      router.push(routes.recruiter.jobPostingQuestions(postingId));
    },
    onError: onStale,
  });

  const back = { href: routes.recruiter.jobPostings, label: "My Job Postings" };

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
          <span className="sr-only">Loading the job posting…</span>
        </div>
      </section>
    );
  if (query.isError) {
    // 400 is a malformed ID in the URL; retrying cannot help, just like a missing posting.
    const notFound = [400, 404].includes(apiError(query.error).status);
    return (
      <section className="mx-auto w-full max-w-5xl space-y-8">
        <PageHeader back={back} title="Job posting" />
        <div
          role="alert"
          className={cn(
            "rounded-xl border p-6",
            !denied && !notFound && "border-destructive/30",
          )}
        >
          <h2 className="text-base font-semibold">
            {denied
              ? "You don't have access to job postings"
              : notFound
                ? "We couldn't find this job posting"
                : "We couldn't load this job posting"}
          </h2>
          <p className="mt-2 text-sm text-muted-foreground">
            {denied
              ? "Checking your account access… If this stays, sign in again with a recruiter account."
              : notFound
                ? "It may have been removed, or it belongs to another recruiter."
                : "Please try again in a moment."}
          </p>
          {!denied && !notFound && (
            <Button
              className={cn("mt-4", pillClass)}
              variant="outline"
              disabled={query.isFetching}
              onClick={() => void query.refetch()}
            >
              Try again
            </Button>
          )}
        </div>
      </section>
    );
  }

  const posting = query.data;
  const locks = postingLocks(posting);
  const hasBank = posting.core_question_count > 0;
  const action = primaryAction(posting);
  const line = statusLine(posting, fmt, capacity.data);
  const generating = save.isPending && save.variables?.generate === true;
  const saveError =
    save.isError && !staleCodes.has(apiError(save.error).code ?? "")
      ? isAccessDenied(save.error)
        ? "You don't have access to edit this posting. Checking your account access…"
        : `${apiError(save.error).message ?? "We couldn't save your changes."} Your changes are still here.`
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
              Created{" "}
              <time dateTime={posting.created_at} title={posting.created_at}>
                {fmt.date(posting.created_at, "calendarDay")}
              </time>
            </span>
            <span>
              Interview slots:{" "}
              <span className="font-medium text-foreground tabular-nums">
                {posting.interview_slot ?? "not funded yet"}
              </span>
            </span>
            {line && (
              <span className="font-medium text-foreground">{line}</span>
            )}
          </>
        }
        description={nextStep(posting)}
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
        <Notice tone="danger" title="An administrator asked for changes">
          {posting.reject_reason && <p>“{posting.reject_reason}”</p>}
          <p className="text-muted-foreground">
            Correct the posting below and save, then resubmit it from its{" "}
            <BankLink postingId={posting.id} />.
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

function BankLink({ postingId }: { postingId: string }) {
  return (
    <Link
      href={routes.recruiter.jobPostingQuestions(postingId)}
      className="font-medium underline underline-offset-4"
    >
      question bank
    </Link>
  );
}

function ReadOnlyPosting({ posting }: { posting: JobPosting }) {
  const details: [string, string][] = [
    ["Seniority", levelText[posting.seniority_level]],
    ["Refinement note", posting.refinement_note || "None"],
    ["Location", posting.location ?? "Not set"],
    ["Employment type", posting.employment_type ?? "Not set"],
    [
      "Salary",
      posting.salary_min === null && posting.salary_max === null
        ? "Not set"
        : `${posting.salary_min ?? "…"} – ${posting.salary_max ?? "…"}`,
    ],
    [
      "Interview length",
      posting.interview_duration === null
        ? "Not set"
        : `${posting.interview_duration} minutes`,
    ],
    ["Interviewer model", posting.required_avatar?.name ?? "Not set"],
    ["Interviewer voice", posting.required_voice?.name ?? "Not set"],
    [
      "Candidates allowed into interview",
      posting.interview_slot === null
        ? "Not set"
        : String(posting.interview_slot),
    ],
  ];
  return (
    <div>
      <FormSection
        title="Details"
        description="What candidates see on the posting and how they are interviewed."
      >
        <dl className="grid gap-x-8 gap-y-4 text-sm sm:grid-cols-2">
          {details.map(([label, value]) => (
            <div key={label} className="space-y-0.5">
              <dt className="text-muted-foreground">{label}</dt>
              <dd className="font-medium [overflow-wrap:anywhere]">{value}</dd>
            </div>
          ))}
        </dl>
      </FormSection>
      <FormSection
        title="Requirements"
        description="Candidates are interviewed on these requirements."
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
                {targetText[row.target]} ·{" "}
                {row.type === "required" ? "Required" : "Preferred"}
              </span>
            </li>
          ))}
        </ul>
      </FormSection>
      <FormSection
        title="Job description"
        description="The text read from the PDF the posting was created with."
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

// The one sentence under the title: what happens next for this posting.
function nextStep(posting: JobPosting) {
  switch (posting.status) {
    case "pending":
      if (posting.review_submitted)
        return "Waiting for an administrator to review it. You can still edit the posting until then.";
      return posting.core_question_count > 0
        ? "The question bank is ready. Review it, then submit the posting for approval."
        : "Confirm the requirements below to generate the question bank. Nothing is sent for approval until you submit.";
    case "rejected":
      return "Make the requested changes, then resubmit the posting for approval.";
    default:
      return posting.status === "open" && posting.interview_slot === null
        ? "Approved, so the posting can no longer be edited. Fund interview slots to open intake to candidates."
        : "Approved, so the posting can no longer be edited.";
  }
}
