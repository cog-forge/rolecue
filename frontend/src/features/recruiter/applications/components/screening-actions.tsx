"use client";

import { useId, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
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
import { applicationErrorMessage } from "../utils/application-errors";
import { RejectDialog } from "./reject-dialog";

// Codes that mean the detail on screen is out of date: the Application was decided, or the
// posting's intake or capacity changed. The detail is reloaded so the screen shows the truth.
const staleCodes = new Set([
  "APPLICATION_NOT_PENDING",
  "INTAKE_NOT_OPEN",
  "INTERVIEW_CAPACITY_FULL",
]);

// Explains why Approve is off while intake is open. The server still decides; Reject stays on.
function approvalBlockReason(posting: ApplicationDetail["job_posting"]) {
  if (posting.interview_slot === null)
    return "Interview capacity is not funded for this posting, so no CV can be approved yet.";
  if (posting.cv_approvals >= posting.interview_slot)
    return "All interview slots for this posting are taken, so no more CVs can be approved.";
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

// The CV decision for a pending Application: Approve (confirmed first) and Reject (optional reason).
// The server decides; the screen only explains what it can see.
export function ScreeningActions({
  postingId,
  applicationId,
  application,
}: {
  postingId: string;
  applicationId: string;
  application: ApplicationDetail;
}) {
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
      gooeyToast.success("CV approved");
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
      gooeyToast.success("CV rejected");
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
      ? "Intake closed first, so this CV was rejected automatically."
      : approve.isError
        ? applicationErrorMessage(approve.error, "We couldn't approve this CV.")
        : reject.isError
          ? applicationErrorMessage(reject.error, "We couldn't reject this CV.")
          : null;
    return failure ? (
      <Notice role="alert" tone="warning" title="No change was made">
        <p className="text-muted-foreground">{failure}</p>
      </Notice>
    ) : null;
  }
  const posting = application.job_posting;
  const intakeOpen = posting.status === "open";
  const intakeClosed =
    posting.status === "intake_closed" || posting.status === "closed";
  const blocked = intakeOpen ? approvalBlockReason(posting) : null;
  const approveError = approve.isError
    ? applicationErrorMessage(
        approve.error,
        "We couldn't approve this CV. Try again.",
      )
    : null;

  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2>Screening</h2>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {!intakeOpen ? (
          intakeClosed ? (
            <Notice tone="locked" title="Intake is closed">
              <p className="text-muted-foreground">
                This posting is no longer taking screening decisions.
              </p>
            </Notice>
          ) : null
        ) : (
          <>
            {blocked && (
              <Notice tone="warning" title="Approval is unavailable">
                <p id={blockedId} className="text-muted-foreground">
                  {blocked}
                </p>
              </Notice>
            )}
            <div className="flex flex-wrap gap-3">
              <Button
                className="h-11"
                disabled={blocked !== null || approve.isPending}
                aria-describedby={blocked ? blockedId : undefined}
                onClick={() => {
                  approve.reset();
                  setApproveOpen(true);
                }}
              >
                Approve CV
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
                Reject CV
              </Button>
            </div>
            {approveError && (
              <p role="alert" className="text-sm text-destructive">
                {approveError}
              </p>
            )}
          </>
        )}
      </CardContent>

      <Dialog
        open={approveOpen}
        onOpenChange={(open) => {
          if (!open && !approve.isPending) setApproveOpen(false);
        }}
      >
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Approve this CV?</DialogTitle>
            <DialogDescription>
              {application.candidate.name} will be cleared for an interview and
              counts toward this posting&apos;s interview limit. When the last
              slot is taken, intake closes and the other pending CVs are
              rejected.
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
              Cancel
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
              {approve.isPending ? "Approving…" : "Confirm approval"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <RejectDialog
        open={rejectOpen}
        pending={reject.isPending}
        error={
          reject.isError
            ? applicationErrorMessage(
                reject.error,
                "We couldn't reject this CV. Try again.",
              )
            : null
        }
        title="Reject this CV?"
        description="This application will be marked rejected. The reason is optional and is stored with the application."
        submitLabel="Reject CV"
        formLabel="Reject CV"
        onCancel={() => setRejectOpen(false)}
        onSubmit={(reason) => reject.mutate(reason)}
      />
    </Card>
  );
}
