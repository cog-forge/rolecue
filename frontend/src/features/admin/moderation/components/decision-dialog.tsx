"use client";

import { useMemo, type ReactNode } from "react";
import { useTranslations } from "next-intl";
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
  createRejectFormSchema,
  rejectReasonLength,
  type RejectValues,
} from "../schemas/moderation-schema";
import { useFormatters } from "@/lib/i18n/use-formatters";

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
  const fmt = useFormatters();
  const t = useTranslations("admin.moderation.dialog");
  const tCommon = useTranslations("common");
  const v = useTranslations("admin.moderation.validation");
  const schema = useMemo(() => createRejectFormSchema(v), [v]);
  const {
    register,
    handleSubmit,
    control,
    formState: { errors },
  } = useForm<RejectValues>({
    resolver: zodResolver(schema),
    defaultValues: { reject_reason: "" },
    mode: "onTouched",
  });
  const reason = useWatch({ control, name: "reject_reason" });
  const reject = decision === "reject";
  const reasonError = errors.reject_reason?.message;
  const emphasize = (chunks: ReactNode) => (
    <span className="font-medium text-foreground">{chunks}</span>
  );

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
            {reject ? t("rejectTitle") : t("approveTitle")}
          </DialogTitle>
          <DialogDescription>
            {t.rich(reject ? "rejectDescription" : "approveDescription", {
              title,
              emphasis: emphasize,
            })}
          </DialogDescription>
        </DialogHeader>
        <form
          noValidate
          aria-label={reject ? t("rejectForm") : t("approveForm")}
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
                {t("reasonLabel")}{" "}
                <span className="text-muted-foreground">{t("required")}</span>
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
                <span>{t("reasonHint")}</span>
                <span className="shrink-0 tabular-nums">
                  {fmt.number(rejectReasonLength(reason ?? ""))}/
                  {fmt.number(MAX_REJECT_REASON)}
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
              {tCommon("actions.cancel")}
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
                  ? t("rejecting")
                  : t("approving")
                : reject
                  ? t("reject")
                  : t("approve")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
