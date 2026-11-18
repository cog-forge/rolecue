"use client";

import { useRef } from "react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { useMutation } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
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
import { ErrorState } from "@/components/ui/error-state";
import { Textarea } from "@/components/ui/textarea";
import { readApiError } from "@/lib/i18n/api-errors";
import { useApiErrorMessage } from "@/lib/i18n/use-api-error-message";
import { reasonSchema } from "../schemas/marketplace-schema";

export type ListingAction =
  "submit" | "delist" | "approve" | "reject" | "suspend";
export function ListingActionDialog({
  action,
  onConfirm,
  onClose,
  onReload,
}: {
  action: ListingAction;
  onConfirm: (reason: string) => Promise<void>;
  onClose: () => void;
  onReload: () => void;
}) {
  const t = useTranslations("marketplace");
  const message = useApiErrorMessage();
  const busy = useRef(false);
  const requiresReason = action === "reject" || action === "suspend";
  const form = useForm({
    defaultValues: { reason: "" },
    resolver: zodResolver(
      z.object({ reason: requiresReason ? reasonSchema : z.string() }),
    ),
    reValidateMode: "onChange",
  });
  const invalid = Boolean(form.formState.errors.reason);
  const mutation = useMutation({
    mutationFn: ({ reason }: { reason: string }) => onConfirm(reason),
    onSuccess: onClose,
    retry: false,
  });
  const stale =
    readApiError(mutation.error).code === "MARKETPLACE_LISTING_STALE";
  function confirm(values: { reason: string }) {
    if (busy.current || stale) return;
    busy.current = true;
    mutation.mutate(values, {
      onSettled: () => {
        busy.current = false;
      },
    });
  }
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !mutation.isPending) onClose();
      }}
    >
      <DialogContent
        className="max-h-[90dvh] overflow-y-auto sm:max-w-lg"
        onPointerDownOutside={(event) => {
          if (mutation.isPending) event.preventDefault();
        }}
        onEscapeKeyDown={(event) => {
          if (mutation.isPending) event.preventDefault();
        }}
      >
        <form
          className="space-y-5"
          noValidate
          onSubmit={(event) => void form.handleSubmit(confirm)(event)}
        >
          <DialogHeader>
            <DialogTitle>{t(`${action}Title`)}</DialogTitle>
            <DialogDescription>{t(`${action}Hint`)}</DialogDescription>
          </DialogHeader>
          {requiresReason && (
            <div className="space-y-2">
              <label
                htmlFor="listing-review-reason"
                className="text-sm font-medium"
              >
                {t("reason")}
              </label>
              <Textarea
                id="listing-review-reason"
                {...form.register("reason")}
                rows={5}
                disabled={mutation.isPending}
                aria-invalid={invalid}
                aria-describedby={invalid ? "reason-error" : undefined}
              />
              {invalid && (
                <p
                  id="reason-error"
                  role="alert"
                  className="text-sm text-destructive"
                >
                  {t("reasonRequired")}
                </p>
              )}
            </div>
          )}
          {mutation.isError && (
            <ErrorState
              variant="compact"
              title={stale ? t("staleTitle") : undefined}
              description={
                stale ? t("staleDescription") : message(mutation.error)
              }
              onRetry={
                stale
                  ? () => {
                      onReload();
                      onClose();
                    }
                  : undefined
              }
              retryLabel={t("reload")}
            />
          )}
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              className="min-h-11"
              disabled={mutation.isPending}
              onClick={onClose}
            >
              {t("cancel")}
            </Button>
            <Button
              type="submit"
              variant={
                action === "delist" || action === "suspend"
                  ? "destructive"
                  : "default"
              }
              className="min-h-11"
              disabled={mutation.isPending || stale}
              aria-busy={mutation.isPending}
            >
              {mutation.isPending && (
                <LoaderCircle
                  aria-hidden
                  className="size-4 motion-safe:animate-spin"
                />
              )}
              {mutation.isPending ? t("actionPending") : t(action)}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
export function DiscardChangesDialog({
  open,
  onStay,
  onLeave,
}: {
  open: boolean;
  onStay: () => void;
  onLeave: () => void;
}) {
  const t = useTranslations("marketplace");
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) onStay();
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("dirtyTitle")}</DialogTitle>
          <DialogDescription>{t("dirtyDescription")}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" className="min-h-11" onClick={onStay}>
            {t("stay")}
          </Button>
          <Button variant="destructive" className="min-h-11" onClick={onLeave}>
            {t("leave")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
