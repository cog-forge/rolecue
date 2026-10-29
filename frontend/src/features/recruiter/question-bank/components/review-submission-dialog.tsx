"use client";

import Link from "next/link";
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
  const user = useSessionUser();
  const weights = useQuery({
    queryKey: evaluationKeys.weights(user.id, postingId),
    queryFn: ({ signal }) => getWeights(postingId, signal),
    refetchOnMount: "always",
    retry: false,
  });
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !pending) onCancel();
      }}
    >
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Review &amp; submit</DialogTitle>
          <DialogDescription>
            Check the posting and evaluation criteria before sending them to an
            administrator. Funding comes after approval.
          </DialogDescription>
        </DialogHeader>
        {posting ? (
          <dl className="grid grid-cols-1 gap-4 rounded-xl border p-4 text-sm sm:grid-cols-2">
            <div className="sm:col-span-2">
              <dt className="text-muted-foreground">Job posting</dt>
              <dd className="mt-1 font-medium">{posting.title}</dd>
            </div>
            <div>
              <dt className="text-muted-foreground">Seniority</dt>
              <dd className="mt-1 capitalize">{posting.seniority_level}</dd>
            </div>
            <div>
              <dt className="text-muted-foreground">Question bank</dt>
              <dd className="mt-1">{questionCount} questions</dd>
            </div>
            <div>
              <dt className="text-muted-foreground">Interviewer</dt>
              <dd className="mt-1">
                {posting.required_avatar?.name ?? "Not selected"}
              </dd>
            </div>
            <div>
              <dt className="text-muted-foreground">Voice</dt>
              <dd className="mt-1">
                {posting.required_voice?.name ?? "Not selected"}
              </dd>
            </div>
            <div className="sm:col-span-2">
              <dt className="text-muted-foreground">Confirmed requirements</dt>
              <dd className="mt-1">
                {posting.requirements.map((item) => item.name).join(", ") ||
                  "No requirements available"}
              </dd>
            </div>
          </dl>
        ) : (
          <p role="alert">
            Posting details are unavailable. Close this review and reload the
            posting.
          </p>
        )}
        <section className="space-y-3 text-sm" aria-label="Evaluation summary">
          <h3 className="font-medium">Evaluation criteria</h3>
          {weights.isPending ? (
            <p role="status">Loading evaluation criteria…</p>
          ) : weights.isError ? (
            <div role="alert">
              <p>We couldn&apos;t load the criteria.</p>
              <Button variant="outline" onClick={() => void weights.refetch()}>
                Retry criteria
              </Button>
            </div>
          ) : (
            <>
              <p className="text-muted-foreground">
                {weights.data.using_defaults
                  ? "Default criteria · equal weights"
                  : "Custom evaluation criteria"}
              </p>
              {!weights.data.editable && (
                <p role="alert">
                  This posting has already been reviewed. Close this dialog and
                  reload the posting to see its current state.
                </p>
              )}
              <dl className="divide-y">
                {weights.data.weights.map((weight) => (
                  <div
                    key={weight.metric_id}
                    className="flex justify-between gap-4 py-2"
                  >
                    <dt>{weight.name}</dt>
                    <dd className="tabular-nums">
                      {Number(weight.percentage.toFixed(2))}
                      {weights.data.total > 0 ? "%" : " weight"}
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
              Edit evaluation criteria
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
            Cancel
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
            {pending
              ? "Submitting…"
              : rejected
                ? "Resubmit for approval"
                : "Submit for approval"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
