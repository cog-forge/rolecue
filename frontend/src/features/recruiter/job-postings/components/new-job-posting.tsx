"use client";

import { useRef } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { gooeyToast } from "goey-toast";
import { routes } from "@/config/routes";
import { Button } from "@/components/ui/button";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { generateBank } from "@/features/recruiter/question-bank/api/question-bank";
import { createPosting, jobPostingKeys } from "../api/job-postings";
import { createBody, emptyPostingValues } from "../schemas/job-posting-schema";
import { apiError, isAccessDenied } from "../utils/api-error";
import { PageHeader, pillClass } from "@/components/layout/page-header";
import { PostingProgress } from "./posting-progress";
import { PostingForm } from "./posting-form";

export function NewJobPosting() {
  const user = useSessionUser();
  const cache = useQueryClient();
  const router = useRouter();
  // Validation is async, so two quick clicks can both reach onSubmit before isPending flips.
  const inFlight = useRef(false);
  const generate = useMutation({
    mutationFn: (posting: { id: string }) => generateBank(posting.id),
    retry: false,
    onSuccess: (_, posting) => {
      void cache.invalidateQueries({
        queryKey: jobPostingKeys.detail(user.id, posting.id),
      });
      void cache.invalidateQueries({ queryKey: jobPostingKeys.lists(user.id) });
      gooeyToast.success("Question bank generated", {
        description: "Review your questions before submitting for approval.",
      });
      router.replace(routes.recruiter.jobPostingQuestions(posting.id));
    },
    onError: (error, posting) => {
      if (isAccessDenied(error))
        void cache.invalidateQueries({ queryKey: ["auth", "me"] });
      const { code } = apiError(error);
      if (code === "QUESTION_BANK_LOCKED") {
        void cache.invalidateQueries({
          queryKey: jobPostingKeys.detail(user.id, posting.id),
        });
        router.replace(routes.recruiter.jobPosting(posting.id));
      }
    },
  });
  const create = useMutation({
    mutationFn: createPosting,
    retry: false,
    onSuccess: (posting) => {
      cache.setQueryData(jobPostingKeys.detail(user.id, posting.id), posting);
      void cache.invalidateQueries({ queryKey: jobPostingKeys.lists(user.id) });
      generate.mutate(posting);
    },
    onError: (error) => {
      if (isAccessDenied(error))
        void cache.invalidateQueries({ queryKey: ["auth", "me"] });
    },
  });

  const submitError = create.isError
    ? isAccessDenied(create.error)
      ? "You don't have access to create job postings. Checking your account access…"
      : `${apiError(create.error).message ?? "We couldn't save the posting."} Your draft is still here.`
    : null;

  return (
    <section className="mx-auto w-full max-w-5xl space-y-8">
      <PageHeader
        back={{ href: routes.recruiter.jobPostings, label: "My Job Postings" }}
        title="New job posting"
        description="Upload the job description and review the requirements RoleCue suggests. Confirming them generates the question bank. Review the questions and evaluation criteria before submitting for approval."
      />
      <PostingProgress
        posting={
          create.data
            ? {
                status: "pending",
                review_submitted: false,
                core_question_count: 0,
              }
            : null
        }
        generating={generate.isPending}
      />
      {create.data ? (
        <GenerationState
          postingId={create.data.id}
          pending={generate.isPending}
          error={
            generate.isError
              ? (apiError(generate.error).message ??
                "We couldn't generate questions.")
              : null
          }
          onRetry={() => {
            if (create.data) generate.mutate(create.data);
          }}
        />
      ) : (
        <PostingForm
          mode="create"
          defaultValues={emptyPostingValues}
          locks={{ textEditable: true, requirementsLocked: false }}
          submitting={create.isPending || create.isSuccess}
          submitError={submitError}
          onSubmit={(values) => {
            if (inFlight.current) return;
            inFlight.current = true;
            create.mutate(createBody(values), {
              onError: () => {
                inFlight.current = false;
              },
            });
          }}
        />
      )}
    </section>
  );
}

// After the posting is saved, generation runs on its own request; a failure keeps the posting and
// retries on the same ID instead of creating it again.
function GenerationState({
  postingId,
  pending,
  error,
  onRetry,
}: {
  postingId: string;
  pending: boolean;
  error: string | null;
  onRetry: () => void;
}) {
  return (
    <div className="space-y-5 rounded-xl border p-6">
      {error ? (
        <div role="alert" className="space-y-1">
          <h2 className="text-base font-semibold">
            Questions weren&apos;t generated
          </h2>
          <p className="text-sm text-muted-foreground">
            {error} Your posting is saved, so retrying only generates the
            questions.
          </p>
        </div>
      ) : (
        <div role="status" className="space-y-1">
          <h2 className="text-base font-semibold">
            <span className="t-shimmer" data-text="Generating questions…">
              Generating questions…
            </span>
          </h2>
          <p className="text-sm text-muted-foreground">
            Your posting is saved. This can take a minute, and it is not
            submitted for approval yet.
          </p>
        </div>
      )}
      <div className="flex flex-wrap items-center gap-2">
        {error && (
          <Button className={pillClass} disabled={pending} onClick={onRetry}>
            Retry generating questions
          </Button>
        )}
        <Button asChild variant="outline" className={pillClass}>
          <Link href={routes.recruiter.jobPosting(postingId)}>
            Open the saved posting
          </Link>
        </Button>
      </div>
    </div>
  );
}
