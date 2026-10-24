"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  applicationKeys,
  listPostingApplications,
} from "@/features/recruiter/applications/api/applications";
import { defaultApplicationFilters } from "@/features/recruiter/applications/utils/application-filters";
import { jobPostingKeys } from "@/features/recruiter/job-postings/api/job-postings";
import { Notice } from "@/features/recruiter/job-postings/components/notice";
import {
  apiError,
  isAccessDenied,
} from "@/features/recruiter/job-postings/utils/api-error";
import {
  capacityKeys,
  closeIntake,
  fundCapacity,
  getCapacity,
} from "../api/capacity";
import type { CapacityState } from "../schemas/capacity-schema";
import { capacityErrorMessage } from "../utils/capacity-errors";
import { CloseIntakeDialog } from "./close-intake-dialog";
import { FundCapacityForm } from "./fund-capacity-form";

// Codes that mean the shown capacity is out of date: reload it so the screen shows the server's truth.
const staleFundingCodes = new Set([
  "INTAKE_NOT_OPEN",
  "INTERVIEW_CAPACITY_ALREADY_FUNDED",
]);

const coins = (amount: number) =>
  `${amount.toLocaleString("en")} ${amount === 1 ? "coin" : "coins"}`;

// The Recruiter's view of one posting's interview capacity. Submitted CVs are shown apart from the
// slots: a submission never uses a slot, only an approval does.
export function CapacityPanel({ postingId }: { postingId: string }) {
  const user = useSessionUser();
  const cache = useQueryClient();
  const key = capacityKeys.detail(user.id, postingId);
  const [closeOpen, setCloseOpen] = useState(false);

  const query = useQuery({
    queryKey: key,
    queryFn: ({ signal }) => getCapacity(postingId, signal),
    refetchOnMount: "always",
    retry: false,
  });
  const state = query.data;
  // Pending and rejected postings take no applications, so there is nothing to count.
  const takesApplications =
    state !== undefined &&
    state.posting_status !== "pending" &&
    state.posting_status !== "rejected";
  const submitted = useQuery({
    // Same key as the applications list with its default filters, so the two share one request.
    queryKey: applicationKeys.posting(
      user.id,
      postingId,
      defaultApplicationFilters,
    ),
    queryFn: ({ signal }) =>
      listPostingApplications(postingId, defaultApplicationFilters, signal),
    enabled: takesApplications,
    retry: false,
  });

  const refreshPosting = () => {
    void cache.invalidateQueries({
      queryKey: jobPostingKeys.detail(user.id, postingId),
    });
  };

  const fund = useMutation({
    mutationFn: (slots: number) => fundCapacity(postingId, slots),
    retry: false,
    onSuccess: (funded) => {
      cache.setQueryData<CapacityState>(key, funded);
      refreshPosting();
    },
    onError: (failure) => {
      if (isAccessDenied(failure))
        void cache.invalidateQueries({ queryKey: ["auth", "me"] });
      const { code } = apiError(failure);
      if (code && staleFundingCodes.has(code))
        void cache.invalidateQueries({ queryKey: key });
    },
  });

  const close = useMutation({
    mutationFn: () => closeIntake(postingId),
    retry: false,
    onSuccess: () => {
      setCloseOpen(false);
      void cache.invalidateQueries({ queryKey: key });
      refreshPosting();
      void cache.invalidateQueries({
        queryKey: jobPostingKeys.lists(user.id),
      });
      void cache.invalidateQueries({ queryKey: applicationKeys.all(user.id) });
    },
    onError: (failure) => {
      if (isAccessDenied(failure))
        void cache.invalidateQueries({ queryKey: ["auth", "me"] });
      // Any refusal may mean the posting moved on; reload it so the dialog shows the current state.
      void cache.invalidateQueries({ queryKey: key });
    },
  });

  if (query.isPending)
    return (
      <Card>
        <CardContent role="status" className="space-y-3 pt-6">
          <Skeleton className="h-6 w-1/3 rounded-md" />
          <Skeleton className="h-24 rounded-xl" />
          <span className="sr-only">Loading interview capacity…</span>
        </CardContent>
      </Card>
    );
  if (query.isError)
    return (
      <CapacityLoadError
        error={query.error}
        retrying={query.isFetching}
        onRetry={() => void query.refetch()}
      />
    );

  const posting = query.data;
  const open = posting.posting_status === "open";
  const closed = posting.posting_status === "closed";

  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2>Interview capacity</h2>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-6 text-sm">
        <section className="space-y-3" aria-labelledby="capacity-slots-heading">
          <h3 id="capacity-slots-heading" className="font-semibold">
            Interview slots
          </h3>
          {posting.funded && posting.interview_slot !== null ? (
            <dl className="grid gap-4 sm:grid-cols-2">
              <CapacityFigure
                label="Funded slots"
                value={posting.interview_slot}
              />
              <CapacityFigure
                label="CVs approved"
                value={posting.cv_approvals}
              />
              <CapacityFigure
                label="Still open for approval"
                value={posting.available_to_approve}
              />
              <CapacityFigure
                label="Interviews started"
                value={posting.consumed}
              />
            </dl>
          ) : (
            <p className="text-muted-foreground">
              Interview slots are not funded yet. No CV can be approved until
              they are.
            </p>
          )}
        </section>

        <section className="space-y-2" aria-labelledby="capacity-cvs-heading">
          <h3 id="capacity-cvs-heading" className="font-semibold">
            CVs submitted
          </h3>
          <SubmittedCount
            takesApplications={takesApplications}
            query={submitted}
          />
          <p className="text-xs text-muted-foreground">
            Submitted CVs do not use interview slots. A slot is used only when a
            CV is approved.
          </p>
        </section>

        {!posting.funded && open && (
          <section
            className="space-y-3"
            aria-labelledby="capacity-fund-heading"
          >
            <h3 id="capacity-fund-heading" className="font-semibold">
              Fund interview slots
            </h3>
            <FundCapacityForm
              pricePerSlot={posting.price_per_slot}
              pending={fund.isPending}
              error={
                fund.isError
                  ? capacityErrorMessage(
                      fund.error,
                      "We couldn't fund the interview slots. Try again.",
                    )
                  : null
              }
              onSubmit={(slots) => fund.mutate(slots)}
            />
          </section>
        )}
        {!posting.funded && !open && !closed && (
          <p className="text-muted-foreground">
            Interview slots can be funded once an administrator approves the
            posting and intake opens.
          </p>
        )}

        {open && (
          <section
            className="space-y-3"
            aria-labelledby="capacity-close-heading"
          >
            <h3 id="capacity-close-heading" className="font-semibold">
              Intake
            </h3>
            <p className="text-muted-foreground">
              Close intake to stop new applications. Pending applications are
              rejected; approved candidates still interview.
            </p>
            <Button
              type="button"
              variant="outline"
              className="h-11 bg-background"
              onClick={() => {
                close.reset();
                setCloseOpen(true);
              }}
            >
              Close intake
            </Button>
          </section>
        )}

        {closed && posting.refunded !== null && (
          <Notice tone="locked" title="Interview slots refunded">
            <p className="text-muted-foreground">
              {posting.refunded === 0
                ? "No coins were returned to your wallet when this posting closed."
                : `${coins(posting.refunded)} were returned to your wallet when this posting closed.`}
            </p>
          </Notice>
        )}

        <CloseIntakeDialog
          open={closeOpen}
          pending={close.isPending}
          error={
            close.isError
              ? capacityErrorMessage(
                  close.error,
                  "We couldn't close intake. Try again.",
                )
              : null
          }
          onCancel={() => setCloseOpen(false)}
          onConfirm={() => close.mutate()}
        />
      </CardContent>
    </Card>
  );
}

function CapacityFigure({ label, value }: { label: string; value: number }) {
  return (
    <div>
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="text-lg font-semibold tabular-nums">{value}</dd>
    </div>
  );
}

function SubmittedCount({
  takesApplications,
  query,
}: {
  takesApplications: boolean;
  query: {
    isPending: boolean;
    isError: boolean;
    data?: { pagination: { total: number } };
  };
}) {
  if (!takesApplications)
    return (
      <p className="text-muted-foreground">
        Candidates can apply once intake opens.
      </p>
    );
  if (query.isPending)
    return (
      <>
        <Skeleton className="h-6 w-16 rounded-md" />
        <span className="sr-only">Loading the number of submitted CVs…</span>
      </>
    );
  if (query.isError || !query.data)
    return (
      <p className="text-muted-foreground">
        We couldn&apos;t load the number of submitted CVs.
      </p>
    );
  return (
    <p className="text-lg font-semibold tabular-nums">
      {query.data.pagination.total}
    </p>
  );
}

function CapacityLoadError({
  error,
  retrying,
  onRetry,
}: {
  error: unknown;
  retrying: boolean;
  onRetry: () => void;
}) {
  const { status, code } = apiError(error);
  const notFound =
    [400, 404].includes(status) || code === "JOB_POSTING_NOT_FOUND";
  const denied = isAccessDenied(error);
  return (
    <Card>
      <CardContent
        role="alert"
        className={cn(
          "space-y-2 pt-6",
          denied || notFound ? "bg-muted/50" : "bg-destructive/5",
        )}
      >
        <h2 className="text-base font-semibold">
          {denied
            ? "You don't have access to interview capacity"
            : notFound
              ? "We couldn't find this job posting"
              : "We couldn't load interview capacity"}
        </h2>
        {!denied && !notFound && (
          <Button
            className="h-11 bg-background"
            variant="outline"
            disabled={retrying}
            onClick={onRetry}
          >
            Try again
          </Button>
        )}
      </CardContent>
    </Card>
  );
}
