"use client";

import { useId, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { zodResolver } from "@hookform/resolvers/zod";
import { LoaderCircle } from "lucide-react";
import { useForm, useWatch } from "react-hook-form";
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
import { Textarea } from "@/components/ui/textarea";
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
import {
  MAX_SCREENING_REASON,
  screeningRejectSchema,
  type ApplicationDetail,
  type Screening,
  type ScreeningRejectValues,
} from "../schemas/application-detail-schema";
import { applicationErrorMessage } from "../utils/application-errors";

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
    const failure = approve.isError
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
        onCancel={() => setRejectOpen(false)}
        onSubmit={(reason) => reject.mutate(reason)}
      />
    </Card>
  );
}

// The reason draft lives in RejectForm, which unmounts with the dialog, so a new open starts empty.
// A failed request keeps the dialog and the draft open.
function RejectDialog({
  open,
  pending,
  error,
  onCancel,
  onSubmit,
}: {
  open: boolean;
  pending: boolean;
  error: string | null;
  onCancel: () => void;
  onSubmit: (reason: string | undefined) => void;
}) {
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next && !pending) onCancel();
      }}
    >
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Reject this CV?</DialogTitle>
          <DialogDescription>
            This application will be marked rejected. The reason is optional and
            is stored with the application.
          </DialogDescription>
        </DialogHeader>
        <RejectForm
          pending={pending}
          error={error}
          onCancel={onCancel}
          onSubmit={onSubmit}
        />
      </DialogContent>
    </Dialog>
  );
}

function RejectForm({
  pending,
  error,
  onCancel,
  onSubmit,
}: {
  pending: boolean;
  error: string | null;
  onCancel: () => void;
  onSubmit: (reason: string | undefined) => void;
}) {
  const reasonId = useId();
  const hintId = useId();
  const errorId = useId();
  const {
    register,
    handleSubmit,
    control,
    formState: { errors },
  } = useForm<ScreeningRejectValues>({
    resolver: zodResolver(screeningRejectSchema),
    defaultValues: { reject_reason: "" },
    mode: "onTouched",
  });
  const reason = useWatch({ control, name: "reject_reason" });
  const reasonError = errors.reject_reason?.message;

  return (
    <form
      noValidate
      aria-label="Reject CV"
      className="space-y-4"
      onSubmit={(event) => {
        event.preventDefault();
        void handleSubmit((values) => {
          const trimmed = values.reject_reason.trim();
          onSubmit(trimmed === "" ? undefined : trimmed);
        })(event);
      }}
    >
      <div className="space-y-2">
        <label htmlFor={reasonId} className="text-sm font-medium">
          Reason for the candidate{" "}
          <span className="text-muted-foreground">(optional)</span>
        </label>
        <Textarea
          id={reasonId}
          rows={4}
          disabled={pending}
          aria-invalid={!!reasonError}
          aria-describedby={[hintId, reasonError ? errorId : ""]
            .filter(Boolean)
            .join(" ")}
          {...register("reject_reason")}
        />
        <p
          id={hintId}
          className="flex justify-between gap-3 text-xs text-muted-foreground"
        >
          <span>Leave it empty to reject without a reason.</span>
          <span className="tabular-nums">
            {Array.from((reason ?? "").trim()).length}/
            {MAX_SCREENING_REASON.toLocaleString("en")}
          </span>
        </p>
        {reasonError && (
          <p id={errorId} role="alert" className="text-xs text-destructive">
            {reasonError}
          </p>
        )}
      </div>
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
      <DialogFooter>
        <Button
          type="button"
          variant="outline"
          className="h-11"
          disabled={pending}
          onClick={onCancel}
        >
          Cancel
        </Button>
        <Button
          type="submit"
          variant="destructive"
          className="h-11"
          disabled={pending}
          aria-busy={pending}
        >
          {pending && (
            <LoaderCircle
              className="size-4 animate-spin motion-reduce:animate-none"
              aria-hidden
            />
          )}
          {pending ? "Rejecting…" : "Reject CV"}
        </Button>
      </DialogFooter>
    </form>
  );
}
