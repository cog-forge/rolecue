"use client";

import { useId } from "react";
import { zodResolver } from "@hookform/resolvers/zod";
import { LoaderCircle } from "lucide-react";
import { useForm, useWatch } from "react-hook-form";
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
  MAX_SCREENING_REASON,
  screeningRejectSchema,
  type ScreeningRejectValues,
} from "../schemas/application-detail-schema";

// The optional reason of a rejection, shared by CV screening and the final decision. The reason
// draft lives in RejectForm, which unmounts with the dialog, so a new open starts empty. A failed
// request keeps the dialog and the draft open.
export function RejectDialog({
  open,
  pending,
  error,
  title,
  description,
  submitLabel,
  formLabel,
  onCancel,
  onSubmit,
}: {
  open: boolean;
  pending: boolean;
  error: string | null;
  title: string;
  description: string;
  submitLabel: string;
  formLabel: string;
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
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <RejectForm
          submitLabel={submitLabel}
          formLabel={formLabel}
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
  submitLabel,
  formLabel,
  pending,
  error,
  onCancel,
  onSubmit,
}: {
  submitLabel: string;
  formLabel: string;
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
      aria-label={formLabel}
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
          {pending ? "Rejecting…" : submitLabel}
        </Button>
      </DialogFooter>
    </form>
  );
}
