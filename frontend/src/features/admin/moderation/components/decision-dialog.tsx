"use client";

import { useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { LoaderCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import {
  MAX_REJECT_REASON,
  rejectFormSchema,
  rejectReasonLength,
  type RejectValues,
} from "../schemas/moderation-schema";

export type Decision = "approve" | "reject";

// One dialog (ADM-05-SUB1) records the decision. Approve only confirms; Reject needs a reason
// the recruiter will read. The reason draft survives a failed request because the dialog stays open.
export function DecisionDialog({
  decision,
  title,
  pending,
  error,
  onCancel,
  onApprove,
  onReject,
}: {
  decision: Decision | null;
  title: string;
  pending: boolean;
  error: string | null;
  onCancel: () => void;
  onApprove: () => void;
  onReject: (reason: string) => void;
}) {
  const {
    register,
    handleSubmit,
    control,
    formState: { errors },
  } = useForm<RejectValues>({
    resolver: zodResolver(rejectFormSchema),
    defaultValues: { reject_reason: "" },
    mode: "onTouched",
  });
  const reason = useWatch({ control, name: "reject_reason" });
  const reject = decision === "reject";
  const reasonError = errors.reject_reason?.message;

  return (
    <Dialog
      open={decision !== null}
      onOpenChange={(open) => {
        if (!open && !pending) onCancel();
      }}
    >
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {reject ? "Reject this posting?" : "Approve this posting?"}
          </DialogTitle>
          <DialogDescription>
            {reject ? (
              <>
                <span className="font-medium text-foreground">{title}</span>{" "}
                won&apos;t be published. The recruiter will see your reason.
              </>
            ) : (
              <>
                <span className="font-medium text-foreground">{title}</span>{" "}
                will be published and candidates can find it.
              </>
            )}
          </DialogDescription>
        </DialogHeader>
        <form
          noValidate
          aria-label={reject ? "Reject posting" : "Approve posting"}
          className="space-y-4"
          onSubmit={(event) => {
            event.preventDefault();
            if (reject)
              void handleSubmit((values) =>
                onReject(values.reject_reason.trim()),
              )(event);
            else onApprove();
          }}
        >
          {reject && (
            <div className="space-y-2">
              <label htmlFor="reject-reason" className="text-sm font-medium">
                Reason for the recruiter{" "}
                <span className="text-muted-foreground">(required)</span>
              </label>
              <Textarea
                id="reject-reason"
                rows={5}
                disabled={pending}
                aria-invalid={!!reasonError}
                aria-describedby={[
                  "reject-reason-hint",
                  reasonError ? "reject-reason-error" : "",
                ]
                  .filter(Boolean)
                  .join(" ")}
                {...register("reject_reason")}
              />
              <p
                id="reject-reason-hint"
                className="flex justify-between gap-3 text-xs text-muted-foreground"
              >
                <span>Be specific so the recruiter knows what to fix.</span>
                <span className="tabular-nums">
                  {rejectReasonLength(reason ?? "").toLocaleString("en")}/
                  {MAX_REJECT_REASON.toLocaleString("en")}
                </span>
              </p>
              {reasonError && (
                <p
                  id="reject-reason-error"
                  role="alert"
                  className="text-xs text-destructive"
                >
                  {reasonError}
                </p>
              )}
            </div>
          )}
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
              variant={reject ? "destructive" : "default"}
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
              {pending
                ? reject
                  ? "Rejecting…"
                  : "Approving…"
                : reject
                  ? "Reject posting"
                  : "Approve posting"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
