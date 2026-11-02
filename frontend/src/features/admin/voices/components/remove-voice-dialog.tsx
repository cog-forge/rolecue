"use client";

import type { ReactNode } from "react";
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
import type { Voice } from "../schemas/voice-schema";
import { useVoiceUsage } from "../utils/voice-labels";

// The outcome is explained before confirming: a voice in use is only deactivated, so past
// interviews and postings keep it; an unused voice is removed for good.
export function RemoveVoiceDialog({
  voice,
  pending,
  error,
  onCancel,
  onConfirm,
}: {
  voice: Voice | null;
  pending: boolean;
  error: string | null;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const t = useTranslations("admin.voices.removeDialog");
  const tCommon = useTranslations("common");
  const usage = useVoiceUsage();
  const inUse = voice?.in_use ?? false;
  const emphasis = (chunks: ReactNode) => (
    <span className="font-medium text-foreground">{chunks}</span>
  );
  const name = voice?.name ?? "";
  return (
    <Dialog
      open={voice !== null}
      onOpenChange={(open) => {
        if (!open && !pending) onCancel();
      }}
    >
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {inUse ? t("deactivateTitle") : t("removeTitle")}
          </DialogTitle>
          <DialogDescription className="[overflow-wrap:anywhere]">
            {inUse
              ? t.rich("inUse", { voice: emphasis, name, usage: usage(voice) })
              : t.rich("unused", { voice: emphasis, name })}
          </DialogDescription>
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
            variant="destructive"
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
            {pending
              ? inUse
                ? t("deactivating")
                : t("removing")
              : inUse
                ? t("deactivateConfirm")
                : t("removeConfirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
