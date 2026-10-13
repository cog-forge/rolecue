"use client";

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
import type { Voice } from "../schemas/voice-schema";

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
  const inUse = voice?.in_use ?? false;
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
            {inUse ? "Deactivate this voice?" : "Remove this voice?"}
          </DialogTitle>
          <DialogDescription className="[overflow-wrap:anywhere]">
            {inUse ? (
              <>
                <span className="font-medium text-foreground">{voice?.name}</span>{" "}
                is used by {usage(voice)}. It will be marked inactive instead of
                removed, so those interviews and postings keep it.
              </>
            ) : (
              <>
                <span className="font-medium text-foreground">{voice?.name}</span>{" "}
                isn&apos;t used anywhere, so it will be removed from the catalog
                permanently.
              </>
            )}
          </DialogDescription>
        </DialogHeader>
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
        <DialogFooter>
          <Button type="button" variant="outline" className="h-11" disabled={pending} onClick={onCancel}>
            Cancel
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
              <LoaderCircle className="size-4 animate-spin motion-reduce:animate-none" aria-hidden />
            )}
            {pending
              ? inUse
                ? "Deactivating…"
                : "Removing…"
              : inUse
                ? "Deactivate voice"
                : "Remove voice"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function usage(voice: Voice | null) {
  if (!voice) return "";
  const parts: string[] = [];
  if (voice.interview_count > 0)
    parts.push(`${voice.interview_count} ${voice.interview_count === 1 ? "interview" : "interviews"}`);
  if (voice.job_posting_count > 0)
    parts.push(`${voice.job_posting_count} ${voice.job_posting_count === 1 ? "job posting" : "job postings"}`);
  return parts.join(" and ");
}
