"use client";

import { useRef, useState, type ChangeEvent, type DragEvent } from "react";
import Link from "next/link";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { FileText, FileUp, LoaderCircle, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { jobBoardKeys } from "@/features/job-board/api/job-board";
import { cn } from "@/lib/utils";
import {
  candidateApplicationKeys,
  uploadApplicationCv,
} from "../api/applications";
import {
  CV_FILE_TYPES,
  CV_MAX_BYTES,
  type CreatedApplication,
} from "../schemas/candidate-application-schema";
import { applyErrorMessage, canRetryApply } from "../utils/api-error";
import type { Formatters } from "@/lib/i18n/formatters";
import { useFormatters } from "@/lib/i18n/use-formatters";
import { useTranslations } from "next-intl";

const isPdf = (file: File) =>
  file.type === "application/pdf" || file.name.toLowerCase().endsWith(".pdf");

function cvFileError(file: File) {
  if (!isPdf(file)) return "Choose a PDF file.";
  if (file.size === 0) return "That file is empty. Choose another PDF.";
  if (file.size > CV_MAX_BYTES)
    return "That PDF is too large. Choose one up to 5 MB.";
  return null;
}

const fileSize = (bytes: number, fmt: Formatters) =>
  bytes < 1024 * 1024
    ? `${fmt.number(bytes / 1024, { maximumFractionDigits: 1 })} KB`
    : `${fmt.number(bytes / 1024 / 1024, { maximumFractionDigits: 1 })} MB`;

// Uploads one CV to the posting, picked or dropped. onApplied receives the new Application; a failed
// upload keeps the same file so "Try again" resends it. onBusyChange lets a dialog stay open meanwhile.
export function CvApplyForm({
  postingId,
  onApplied,
  onBusyChange,
}: {
  postingId: string;
  onApplied: (created: CreatedApplication) => void;
  onBusyChange?: (busy: boolean) => void;
}) {
  const fmt = useFormatters();
  const errorsT = useTranslations("errors");
  const user = useSessionUser();
  const cache = useQueryClient();
  const input = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const [progress, setProgress] = useState(0);
  const [dragging, setDragging] = useState(false);

  const upload = useMutation({
    mutationFn: (cv: File) => uploadApplicationCv(postingId, cv, setProgress),
    onMutate: () => {
      setProgress(0);
      onBusyChange?.(true);
    },
    onSettled: () => onBusyChange?.(false),
    onSuccess: (created) => {
      void cache.invalidateQueries({
        queryKey: candidateApplicationKeys.all(user.id),
      });
      // The posting now says "already applied", so the board shows the Applied state.
      void cache.invalidateQueries({ queryKey: jobBoardKeys.details(user.id) });
      onApplied(created);
    },
  });

  const busy = upload.isPending;
  const failed = upload.isError;
  const retryable = failed && canRetryApply(upload.error);
  const message = failed ? applyErrorMessage(upload.error, errorsT) : fileError;

  const pick = (picked: File | null) => {
    upload.reset();
    if (!picked) return;
    const error = cvFileError(picked);
    setFile(error ? null : picked);
    setFileError(error);
  };
  const choose = (event: ChangeEvent<HTMLInputElement>) => {
    pick(event.target.files?.[0] ?? null);
    // The same file can be chosen again after an error.
    event.target.value = "";
  };
  const drop = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    setDragging(false);
    if (!busy) pick(event.dataTransfer.files[0] ?? null);
  };

  return (
    <div className="space-y-4">
      <input
        ref={input}
        id="cv-file"
        type="file"
        accept={CV_FILE_TYPES}
        className="sr-only"
        tabIndex={-1}
        aria-hidden
        disabled={busy}
        onChange={choose}
      />
      <div
        onDragOver={(event) => {
          event.preventDefault();
          if (!busy) setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={drop}
        className={cn(
          "flex flex-col items-center gap-3 rounded-xl border border-dashed px-4 py-6 text-center transition-colors duration-[var(--duration-quick)] motion-reduce:transition-none",
          dragging && "border-foreground bg-muted/60",
        )}
      >
        {file ? (
          <p className="flex items-center gap-2 text-sm font-medium [overflow-wrap:anywhere]">
            <FileText className="size-4 shrink-0" aria-hidden />
            <span>{file.name}</span>
            <span className="font-normal text-muted-foreground">
              {fileSize(file.size, fmt)}
            </span>
          </p>
        ) : (
          <FileUp className="size-6 text-muted-foreground" aria-hidden />
        )}
        <Button
          type="button"
          variant="outline"
          className="h-11"
          disabled={busy}
          aria-describedby="cv-hint"
          onClick={() => input.current?.click()}
        >
          {file ? "Choose another PDF" : "Choose your CV (PDF)"}
        </Button>
        <p id="cv-hint" className="text-xs text-muted-foreground">
          Or drop it here. One PDF with selectable text, up to 5 MB.
        </p>
      </div>

      {busy && (
        <div className="space-y-2">
          <div
            role="progressbar"
            aria-label="CV upload progress"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={progress}
            aria-valuetext={`${progress}%`}
            className="h-2 w-full overflow-hidden rounded-full bg-muted"
          >
            <div
              className="h-full bg-foreground transition-[width] duration-200 motion-reduce:transition-none"
              style={{ width: `${progress}%` }}
            />
          </div>
          <p role="status" className="flex items-center gap-2 text-sm">
            <LoaderCircle
              className="size-4 animate-spin motion-reduce:animate-none"
              aria-hidden
            />
            {progress < 100
              ? `Uploading your CV… ${progress}%`
              : "Saving your CV…"}
          </p>
        </div>
      )}

      {message && (
        <div role="alert" className="space-y-2 text-sm text-destructive">
          <p>{message}</p>
          {failed && !retryable && (
            <Link
              href={routes.applications}
              className="inline-flex min-h-11 items-center font-medium text-foreground underline underline-offset-4 outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
            >
              View my applications
            </Link>
          )}
        </div>
      )}

      <Button
        type="button"
        className="h-11 w-full"
        disabled={!file || busy || (failed && !retryable)}
        aria-busy={busy}
        onClick={() => file && upload.mutate(file)}
      >
        {retryable ? (
          <>
            <RotateCcw aria-hidden />
            Try again
          </>
        ) : busy ? (
          "Sending…"
        ) : (
          "Send application"
        )}
      </Button>
    </div>
  );
}
