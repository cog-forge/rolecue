"use client";

import { useEffect, useId, useState } from "react";
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
import { applicationKeys, decideApplication } from "../api/applications";
import type { ApplicationDetail } from "../schemas/application-detail-schema";
import { formatDeadline } from "../utils/application-dates";
import { applicationErrorMessage } from "../utils/application-errors";
import { RejectDialog } from "./reject-dialog";

// Codes that mean the detail on screen is out of date: the Application left the interviewed state,
// the final review is still locked, or the Application is gone. The detail is reloaded.
const staleCodes = new Set([
  "APPLICATION_NOT_INTERVIEWED",
  "FINAL_REVIEW_LOCKED",
  "APPLICATION_NOT_FOUND",
]);
// setTimeout wraps past 2^31-1 ms, which would fire straight away.
const MAX_TIMER_DELAY_MS = 2 ** 31 - 1;
// A reload that still finds the gate locked (browser clock ahead of the server) retries this often.
const UNLOCK_RETRY_MS = 3000;
const MAX_UNLOCK_RELOADS = 5;

// The final Approve / Reject for an interviewed Application. The gate comes from the server
// (final_review.can_decide, opens_at); the screen only shows it. Nothing changes until the server
// confirms, after which the detail is reloaded.
export function FinalDecision({
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
  const lockedId = useId();
  const review = application.final_review ?? null;
  const decidable = review?.can_decide === true;
  const opensAt = review?.opens_at ?? null;

  // The controls unlock on their own when opens_at arrives: reload the detail then. The server clock
  // decides, so a browser clock that runs ahead can reload too early; the detail is then still locked
  // and the timer re-arms a few seconds later, a bounded number of times.
  const [reloads, setReloads] = useState(0);
  useEffect(() => {
    if (decidable || !opensAt || reloads > MAX_UNLOCK_RELOADS) return;
    const delay = Math.min(
      MAX_TIMER_DELAY_MS,
      Math.max(
        reloads === 0 ? 1000 : UNLOCK_RETRY_MS,
        Date.parse(opensAt) - Date.now(),
      ),
    );
    const timer = window.setTimeout(() => {
      void cache
        .invalidateQueries({
          queryKey: applicationKeys.detail(user.id, postingId, applicationId),
        })
        .then(() => setReloads((count) => count + 1));
    }, delay);
    return () => window.clearTimeout(timer);
  }, [cache, user.id, postingId, applicationId, decidable, opensAt, reloads]);

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
    mutationFn: () => decideApplication(postingId, applicationId, "approve"),
    retry: false,
    onSuccess: () => {
      refresh();
      setApproveOpen(false);
      gooeyToast.success("Application approved");
    },
    onError: (failure) => {
      setApproveOpen(false);
      onFailure(failure);
    },
  });

  const reject = useMutation({
    mutationFn: (reason: string | undefined) =>
      decideApplication(postingId, applicationId, "reject", reason),
    retry: false,
    onSuccess: () => {
      refresh();
      setRejectOpen(false);
      gooeyToast.success("Application rejected");
    },
    onError: onFailure,
  });

  const approveError = approve.isError
    ? applicationErrorMessage(
        approve.error,
        "We couldn't approve this application. Try again.",
      )
    : null;

  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2>Final decision</h2>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {!decidable && (
          <Notice tone="locked" title="Final decision is not open yet">
            <p id={lockedId} className="text-muted-foreground">
              {opensAt
                ? `Decisions open ${formatDeadline(opensAt)}, once the interview deadlines for this posting have passed.`
                : "Decisions are not open for this application yet."}
            </p>
          </Notice>
        )}
        <div className="flex flex-wrap gap-3">
          <Button
            className="h-11"
            disabled={!decidable || approve.isPending}
            aria-describedby={decidable ? undefined : lockedId}
            onClick={() => {
              approve.reset();
              setApproveOpen(true);
            }}
          >
            Approve application
          </Button>
          <Button
            variant="outline"
            className="h-11"
            disabled={!decidable || reject.isPending}
            aria-describedby={decidable ? undefined : lockedId}
            onClick={() => {
              reject.reset();
              setRejectOpen(true);
            }}
          >
            Reject application
          </Button>
        </div>
        {approveError && (
          <p role="alert" className="text-sm text-destructive">
            {approveError}
          </p>
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
            <DialogTitle>Approve this application?</DialogTitle>
            <DialogDescription>
              {application.candidate.name}&apos;s application will be marked
              approved.
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
                "We couldn't reject this application. Try again.",
              )
            : null
        }
        title="Reject this application?"
        description="This application will be marked rejected. The reason is optional and is stored with the application."
        submitLabel="Reject application"
        formLabel="Reject application"
        onCancel={() => setRejectOpen(false)}
        onSubmit={(reason) => reject.mutate(reason)}
      />
    </Card>
  );
}
