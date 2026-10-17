"use client";

import { useRef } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft } from "lucide-react";
import { gooeyToast } from "goey-toast";
import { routes } from "@/config/routes";
import { cn } from "@/lib/utils";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { createPosting, jobPostingKeys } from "../api/job-postings";
import { createBody, emptyPostingValues } from "../schemas/job-posting-schema";
import { apiError, isAccessDenied } from "../utils/api-error";
import { headerBandClass } from "./notice";
import { PostingForm } from "./posting-form";

export function NewJobPosting() {
  const user = useSessionUser();
  const cache = useQueryClient();
  const router = useRouter();
  // Validation is async, so two quick clicks can both reach onSubmit before isPending flips.
  const inFlight = useRef(false);
  const create = useMutation({
    mutationFn: createPosting,
    retry: false,
    onSuccess: (posting) => {
      cache.setQueryData(jobPostingKeys.detail(user.id, posting.id), posting);
      void cache.invalidateQueries({ queryKey: jobPostingKeys.lists(user.id) });
      gooeyToast.success("Submitted for approval", {
        description:
          "An administrator reviews the posting before candidates can see it.",
      });
      router.replace(routes.recruiter.jobPosting(posting.id));
    },
    onError: (error) => {
      if (isAccessDenied(error))
        void cache.invalidateQueries({ queryKey: ["auth", "me"] });
    },
  });

  const submitError = create.isError
    ? isAccessDenied(create.error)
      ? "You don't have access to create job postings. Checking your account access…"
      : `${apiError(create.error).message ?? "We couldn't submit the posting."} Your draft is still here.`
    : null;

  return (
    <section className="mx-auto w-full max-w-4xl space-y-6">
      <Link
        href={routes.recruiter.jobPostings}
        className="inline-flex min-h-11 items-center gap-1 rounded-md text-sm font-medium text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        <ArrowLeft className="size-4" aria-hidden />
        Back to my job postings
      </Link>
      <header className={cn(headerBandClass, "space-y-2")}>
        <p className="text-xs font-semibold tracking-[0.14em] text-workspace-accent uppercase">
          Recruitment
        </p>
        <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
          Create job posting
        </h1>
        <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
          Paste the job description, review the requirements RoleCue suggests,
          then submit the posting for approval.
        </p>
      </header>
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
    </section>
  );
}
