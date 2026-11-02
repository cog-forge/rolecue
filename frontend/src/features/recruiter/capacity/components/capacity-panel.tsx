"use client";

import Link from "next/link";
import { routes } from "@/config/routes";
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
import { getWallet, walletKeys } from "@/features/billing/api/wallet";
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
import { useFormatters } from "@/lib/i18n/use-formatters";
import { useTranslations } from "next-intl";

// Codes that mean the shown capacity is out of date: reload it so the screen shows the server's truth.
const staleFundingCodes = new Set([
  "INTAKE_NOT_OPEN",
  "INTERVIEW_CAPACITY_ALREADY_FUNDED",
]);

// The Recruiter's view of one posting's interview capacity. Submitted CVs are shown apart from the
// slots: a submission never uses a slot, only an approval does.
export function CapacityPanel({ postingId }: { postingId: string }) {
  const fmt = useFormatters();
  const t = useTranslations("recruiter.capacity");
  const errorsT = useTranslations("errors");
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
  const wallet = useQuery({
    queryKey: walletKeys.wallet(user.id),
    queryFn: ({ signal }) => getWallet(signal),
    enabled: query.data?.posting_status === "open" && !query.data.funded,
    refetchOnWindowFocus: true,
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
      // Funding spends coins, so the wallet balance in the header is stale until it is refetched.
      void cache.invalidateQueries({ queryKey: walletKeys.all(user.id) });
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
      // Closing can end the posting and return unused slots to the wallet.
      void cache.invalidateQueries({ queryKey: walletKeys.all(user.id) });
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
          <span className="sr-only">{t("loading")}</span>
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
    <Card id="interview-capacity">
      <CardHeader>
        <CardTitle>
          <h2>{t("title")}</h2>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-6 text-sm">
        <section className="space-y-3" aria-labelledby="capacity-slots-heading">
          <h3 id="capacity-slots-heading" className="font-semibold">
            {t("slots.title")}
          </h3>
          {posting.funded && posting.interview_slot !== null ? (
            <dl className="grid gap-4 sm:grid-cols-2">
              <CapacityFigure
                label={t("slots.funded")}
                value={posting.interview_slot}
              />
              <CapacityFigure
                label={t("slots.approved")}
                value={posting.cv_approvals}
              />
              <CapacityFigure
                label={t("slots.openForApproval")}
                value={posting.available_to_approve}
              />
              <CapacityFigure
                label={t("slots.started")}
                value={posting.consumed}
              />
            </dl>
          ) : (
            <p className="text-muted-foreground">
              {open ? t("slots.approvedUnfunded") : t("slots.notFunded")}
            </p>
          )}
        </section>

        <section className="space-y-2" aria-labelledby="capacity-cvs-heading">
          <h3 id="capacity-cvs-heading" className="font-semibold">
            {t("cvs.title")}
          </h3>
          <SubmittedCount
            takesApplications={takesApplications}
            query={submitted}
          />
          {open && !posting.funded && (
            <p className="text-muted-foreground">
              {t("cvs.applyAfterFunding")}
            </p>
          )}
          <p className="text-xs text-muted-foreground">{t("cvs.note")}</p>
        </section>

        {!posting.funded && open && (
          <section
            className="space-y-3"
            aria-labelledby="capacity-fund-heading"
          >
            <h3 id="capacity-fund-heading" className="font-semibold">
              {t("fund.title")}
            </h3>
            <p className="text-muted-foreground">{t("fund.note")}</p>
            <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg bg-muted/40 p-3">
              <p>
                {wallet.isSuccess
                  ? t("fund.walletBalance", {
                      amount: fmt.coins(wallet.data.balance),
                    })
                  : wallet.isPending
                    ? t("fund.walletLoading")
                    : t("fund.walletUnavailable")}
              </p>
              <Link
                href={routes.recruiter.billing}
                className="inline-flex min-h-11 items-center text-sm font-medium underline underline-offset-4"
              >
                {t("fund.viewWallet")}
              </Link>
            </div>
            <FundCapacityForm
              pricePerSlot={posting.price_per_slot}
              pending={fund.isPending}
              error={
                fund.isError
                  ? capacityErrorMessage(
                      fund.error,
                      errorsT,
                      t,
                      t("fund.failed"),
                    )
                  : null
              }
              onSubmit={(slots) => fund.mutate(slots)}
            />
          </section>
        )}
        {!posting.funded && !open && !closed && (
          <p className="text-muted-foreground">
            {posting.posting_status === "intake_closed"
              ? t("fund.intakeClosed")
              : t("fund.awaitingApproval")}
          </p>
        )}

        {open && (
          <section
            className="space-y-3"
            aria-labelledby="capacity-close-heading"
          >
            <h3 id="capacity-close-heading" className="font-semibold">
              {t("intake.title")}
            </h3>
            <p className="text-muted-foreground">{t("intake.description")}</p>
            <Button
              type="button"
              variant="outline"
              className="h-11 bg-background"
              onClick={() => {
                close.reset();
                setCloseOpen(true);
              }}
            >
              {t("intake.close")}
            </Button>
          </section>
        )}

        {closed && posting.interview_slot === null && (
          <Notice tone="locked" title={t("closedUnfunded.title")}>
            <p className="text-muted-foreground">
              {t("closedUnfunded.description")}
            </p>
          </Notice>
        )}

        {closed &&
          posting.interview_slot !== null &&
          posting.refunded !== null && (
            <Notice tone="locked" title={t("refunded.title")}>
              <p className="text-muted-foreground">
                {posting.refunded === 0
                  ? t("refunded.none")
                  : t("refunded.returned", {
                      amount: fmt.coins(posting.refunded),
                    })}
              </p>
            </Notice>
          )}

        <CloseIntakeDialog
          open={closeOpen}
          pending={close.isPending}
          error={
            close.isError
              ? capacityErrorMessage(close.error, errorsT, t, t("closeFailed"))
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
  const fmt = useFormatters();
  return (
    <div>
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="text-lg font-semibold tabular-nums">
        {fmt.number(value)}
      </dd>
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
  const t = useTranslations("recruiter.capacity.submitted");
  const fmt = useFormatters();
  if (!takesApplications)
    return <p className="text-muted-foreground">{t("notOpen")}</p>;

  if (query.isPending)
    return (
      <>
        <Skeleton className="h-6 w-16 rounded-md" />
        <span className="sr-only">{t("loading")}</span>
      </>
    );
  if (query.isError || !query.data)
    return <p className="text-muted-foreground">{t("failed")}</p>;
  return (
    <p className="text-lg font-semibold tabular-nums">
      {fmt.number(query.data.pagination.total)}
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
  const t = useTranslations("recruiter.capacity.loadError");
  const tCommon = useTranslations("common");
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
          {denied ? t("denied") : notFound ? t("notFound") : t("failed")}
        </h2>
        {!denied && !notFound && (
          <Button
            className="h-11 bg-background"
            variant="outline"
            disabled={retrying}
            onClick={onRetry}
          >
            {tCommon("actions.tryAgain")}
          </Button>
        )}
      </CardContent>
    </Card>
  );
}
