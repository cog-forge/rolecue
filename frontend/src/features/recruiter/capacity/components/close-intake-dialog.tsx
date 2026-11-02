"use client";

import { LoaderCircle } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

// Confirms Close Intake before the call. Closing is the only intake action; there is no reopen.
export function CloseIntakeDialog({
  open,
  pending,
  error,
  onCancel,
  onConfirm,
}: {
  open: boolean;
  pending: boolean;
  error: string | null;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const t = useTranslations("recruiter.capacity.closeDialog");
  const tCommon = useTranslations("common");
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next && !pending) onCancel();
      }}
    >
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("title")}</DialogTitle>
          <DialogDescription>{t("description")}</DialogDescription>
        </DialogHeader>
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
            type="button"
            className="h-11"
            disabled={pending}
            aria-busy={pending}
            onClick={onConfirm}
          >
            {pending && (
              <LoaderCircle
                className="size-4 animate-spin motion-reduce:animate-none"
                aria-hidden
              />
            )}
            {pending ? t("pending") : t("confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
