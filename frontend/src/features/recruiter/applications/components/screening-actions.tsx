"use client";

import { useId, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { LoaderCircle } from "lucide-react";
import { gooeyToast } from "goey-toast";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { Notice } from "@/features/recruiter/job-postings/components/notice";
import {
  apiError,
  isAccessDenied,
} from "@/features/recruiter/job-postings/utils/api-error";
import {
  applicationKeys,
  approveApplicationCv,
  rejectApplicationCv,
} from "../api/applications";
import type {
  ApplicationDetail,
  Screening,
} from "../schemas/application-detail-schema";
import { useApplicationErrorMessage } from "../utils/application-errors";
import { RejectDialog } from "./reject-dialog";

// Codes that mean the detail on screen is out of date: the Application was decided, or the
// posting's intake or capacity changed. The detail is reloaded so the screen shows the truth.
const staleCodes = new Set([
  "APPLICATION_NOT_PENDING",
  "INTAKE_NOT_OPEN",
  "INTERVIEW_CAPACITY_FULL",
]);

// Why Approve is off while intake is open. The server still decides; Reject stays on.
function approvalBlock(posting: ApplicationDetail["job_posting"]) {
  if (posting.interview_slot === null) return "blockedNotFunded";
  if (posting.cv_approvals >= posting.interview_slot) return "blockedFull";
  return null;
}

// Writes the screening response into the detail at once; the refetch that follows confirms it.
function applyScreening(
  current: ApplicationDetail,
  screening: Screening,
  approved: boolean,
): ApplicationDetail {
  return {
    ...current,
    status: screening.status,
    interview_eligible: screening.status === "interview_eligible",
    interview_deadline: screening.interview_deadline,
    reject_reason: screening.reject_reason,
    updated_at: screening.updated_at,
    job_posting: {
      ...current.job_posting,
      status: screening.job_posting_status,
      cv_approvals: current.job_posting.cv_approvals + (approved ? 1 : 0),
    },
  };
}

// Screening is repeated many times in a row, so Approve asks for confirmation only when it takes
// the last interview slot: that approval also closes intake and rejects the other pending CVs.
function takesLastSlot(posting: ApplicationDetail["job_posting"]) {
  return (
    posting.interview_slot !== null &&
    posting.cv_approvals + 1 >= posting.interview_slot
  );
}

// The CV decision for a pending Application: Approve and Reject (optional reason). The server
// decides; the screen only explains what it can see. "inline" drops the card for the review panel.
export function ScreeningActions({
  postingId,
  applicationId,
  application,
  layout = "card",
  onDecided,
}: {
  postingId: string;
  applicationId: string;
  application: ApplicationDetail;
  layout?: "card" | "inline";
  onDecided?: (decision: "approved" | "rejected") => void;
}) {
  const t = useTranslations("recruiter.applications");
  const tCommon = useTranslations("common");
  const errorMessage = useApplicationErrorMessage();
  const user = useSessionUser();
  const cache = useQueryClient();
  const [approveOpen, setApproveOpen] = useState(false);
  const [rejectOpen, setRejectOpen] = useState(false);
  const blockedId = useId();
  const detailKey = applicationKeys.detail(user.id, postingId, applicationId);

  const refresh = () => {
    void cache.invalidateQueries({ queryKey: detailKey });
    void cache.invalidateQueries({
      queryKey: [...applicationKeys.all(user.id), "own"],
    });
    void cache.invalidateQueries({
      queryKey: [...applicationKeys.all(user.id), "posting"],
    });
  };
  const onFailure = (failure: unknown) => {
    // A role or lock change shows up as 401/403; let the session gate re-check access.
    if (isAccessDenied(failure))
      void cache.invalidateQueries({ queryKey: ["auth", "me"] });
    const { code } = apiError(failure);
    if (code && staleCodes.has(code))
      void cache.invalidateQueries({ queryKey: detailKey });
  };

  const approve = useMutation({
    mutationFn: () => approveApplicationCv(postingId, applicationId),
    retry: false,
    onSuccess: (screening) => {
      cache.setQueryData<ApplicationDetail>(detailKey, (current) =>
        current ? applyScreening(current, screening, true) : current,
      );
      refresh();
      setApproveOpen(false);
      gooeyToast.success(t("screening.approvedToast"));
      onDecided?.("approved");
    },
    onError: (failure) => {
      setApproveOpen(false);
      onFailure(failure);
    },
  });

  const reject = useMutation({
    mutationFn: (reason: string | undefined) =>
      rejectApplicationCv(postingId, applicationId, reason),
    retry: false,
    onSuccess: (screening) => {
      cache.setQueryData<ApplicationDetail>(detailKey, (current) =>
        current ? applyScreening(current, screening, false) : current,
      );
      refresh();
      setRejectOpen(false);
      gooeyToast.success(t("screening.rejectedToast"));
      onDecided?.("rejected");
    },
    onError: onFailure,
  });

  if (application.status !== "pending") {
    // The reload after a conflict (for example APPLICATION_NOT_PENDING) can leave the Application
    // decided; the reason for the failed click stays visible instead of vanishing with the card.
    // An approve that lost to an intake close means the CV was rejected automatically; the posting
    // can be intake_closed because the last slot was filled or because the Recruiter closed it.
    const closedFirst =
      approve.isError &&
      apiError(approve.error).code === "APPLICATION_NOT_PENDING" &&
      application.job_posting.status !== "open";
    const failure = closedFirst
      ? t("screening.closedFirst")
      : approve.isError
        ? errorMessage(approve.error, t("screening.approveFailed"))
        : reject.isError
          ? errorMessage(reject.error, t("screening.rejectFailed"))
          : null;
    return failure ? (
      <Notice role="alert" tone="warning" title={t("screening.noChangeTitle")}>
        <p className="text-muted-foreground">{failure}</p>
      </Notice>
    ) : null;
  }
  const posting = application.job_posting;
  const intakeOpen = posting.status === "open";
  const intakeClosed =
    posting.status === "intake_closed" || posting.status === "closed";
  const blocked = intakeOpen ? approvalBlock(posting) : null;
  const approveError = approve.isError
    ? errorMessage(approve.error, t("screening.approveFailedRetry"))
    : null;

  const controls = (
    <>
      {!intakeOpen ? (
        intakeClosed ? (
          <Notice tone="locked" title={t("screening.intakeClosedTitle")}>
            <p className="text-muted-foreground">
              {t("screening.intakeClosedBody")}
            </p>
          </Notice>
        ) : null
      ) : (
        <>
          {blocked && (
            <Notice tone="warning" title={t("screening.unavailableTitle")}>
              <p id={blockedId} className="text-muted-foreground">
                {t(`screening.${blocked}`)}
              </p>
            </Notice>
          )}
          <div className="flex flex-wrap gap-3">
            <Button
              className="h-11"
              disabled={blocked !== null || approve.isPending}
              aria-describedby={blocked ? blockedId : undefined}
              aria-busy={approve.isPending && !approveOpen}
              onClick={() => {
                approve.reset();
                if (takesLastSlot(posting)) setApproveOpen(true);
                else approve.mutate();
              }}
            >
              {approve.isPending && !approveOpen && (
                <LoaderCircle
                  className="size-4 animate-spin motion-reduce:animate-none"
                  aria-hidden
                />
              )}
              {t("screening.approve")}
            </Button>
            <Button
              variant="outline"
              className="h-11"
              disabled={reject.isPending}
              onClick={() => {
                reject.reset();
                setRejectOpen(true);
              }}
            >
              {t("screening.reject")}
            </Button>
          </div>
          {approveError && (
            <p role="alert" className="text-sm text-destructive">
              {approveError}
            </p>
          )}
        </>
      )}

      <Dialog
        open={approveOpen}
        onOpenChange={(open) => {
          if (!open && !approve.isPending) setApproveOpen(false);
        }}
      >
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{t("screening.lastSlotTitle")}</DialogTitle>
            <DialogDescription>
              {t("screening.lastSlotBody", {
                name: application.candidate.name,
              })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              className="h-11"
              disabled={approve.isPending}
              onClick={() => setApproveOpen(false)}
            >
              {tCommon("actions.cancel")}
            </Button>
            <Button
              type="button"
              className="h-11"
              disabled={approve.isPending}
              aria-busy={approve.isPending}
              onClick={() => approve.mutate()}
            >
              {approve.isPending && (
                <LoaderCircle
                  className="size-4 animate-spin motion-reduce:animate-none"
                  aria-hidden
                />
              )}
              {approve.isPending
                ? t("dialog.approving")
                : t("dialog.confirmApproval")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <RejectDialog
        open={rejectOpen}
        pending={reject.isPending}
        error={
          reject.isError
            ? errorMessage(reject.error, t("screening.rejectFailedRetry"))
            : null
        }
        title={t("screening.rejectTitle")}
        description={t("reject.description")}
        submitLabel={t("screening.reject")}
        formLabel={t("screening.reject")}
        onCancel={() => setRejectOpen(false)}
        onSubmit={(reason) => reject.mutate(reason)}
      />
    </>
  );
  if (layout === "inline")
    return (
      <section aria-label={t("screening.heading")} className="space-y-3">
        {controls}
      </section>
    );
  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2>{t("screening.heading")}</h2>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">{controls}</CardContent>
    </Card>
  );
}
