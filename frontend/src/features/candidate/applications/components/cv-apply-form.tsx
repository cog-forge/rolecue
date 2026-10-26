"use client";

import { useRef, useState, type ChangeEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { FileUp, LoaderCircle, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { jobBoardKeys } from "@/features/job-board/api/job-board";
import {
  candidateApplicationKeys,
  uploadApplicationCv,
} from "../api/applications";
import {
  CV_FILE_TYPES,
  CV_MAX_BYTES,
} from "../schemas/candidate-application-schema";
import { applicationHref } from "../utils/application-links";
import { applyErrorMessage, canRetryApply } from "../utils/api-error";

const isPdf = (file: File) =>
  file.type === "application/pdf" || file.name.toLowerCase().endsWith(".pdf");

function cvFileError(file: File) {
  if (!isPdf(file)) return "Choose a PDF file.";
  if (file.size === 0) return "That file is empty. Choose another PDF.";
  if (file.size > CV_MAX_BYTES)
    return "That PDF is too large. Choose one up to 5 MB.";
  return null;
}

// Uploads one CV to the posting. Success moves the Candidate to the Application status screen;
// a failed upload keeps the same file so "Try again" resends it.
export function CvApplyForm({ postingId }: { postingId: string }) {
  const user = useSessionUser();
  const cache = useQueryClient();
  const router = useRouter();
  const input = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const [progress, setProgress] = useState(0);

  const upload = useMutation({
    mutationFn: (cv: File) => uploadApplicationCv(postingId, cv, setProgress),
    onMutate: () => setProgress(0),
    onSuccess: (created) => {
      void cache.invalidateQueries({
        queryKey: candidateApplicationKeys.all(user.id),
      });
      // The posting now says "already applied", so the next visit shows the Applied state.
      void cache.invalidateQueries({ queryKey: jobBoardKeys.details(user.id) });
      router.push(applicationHref(created.id));
    },
  });

  const busy = upload.isPending;
  const failed = upload.isError;
  const retryable = failed && canRetryApply(upload.error);
  const message = failed ? applyErrorMessage(upload.error) : fileError;

  const choose = (event: ChangeEvent<HTMLInputElement>) => {
    const picked = event.target.files?.[0] ?? null;
    upload.reset();
    if (!picked) return;
    const error = cvFileError(picked);
    setFile(error ? null : picked);
    setFileError(error);
  };

  const send = () => {
    if (file) upload.mutate(file);
  };

  return (
    <div className="space-y-3 border-t pt-4">
      <h3 className="text-sm font-semibold">Apply with your CV</h3>
      <p id="cv-hint" className="text-xs text-muted-foreground">
        One PDF with selectable text, up to 5 MB. The recruiter reviews your CV
        first; no interview starts from this step.
      </p>
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
      <div className="flex flex-wrap items-center gap-2">
        <Button
          type="button"
          variant="outline"
          className="h-11"
          disabled={busy}
          aria-describedby="cv-hint"
          onClick={() => input.current?.click()}
        >
          <FileUp aria-hidden />
          {file ? "Choose another PDF" : "Choose your CV (PDF)"}
        </Button>
        {file && !busy && (
          <span className="text-sm font-medium [overflow-wrap:anywhere]">
            {file.name}
          </span>
        )}
      </div>

      {file && !busy && !(failed && !retryable) && (
        <Button type="button" className="h-11 w-full" onClick={send}>
          {retryable ? (
            <>
              <RotateCcw aria-hidden />
              Try again
            </>
          ) : (
            "Apply with this CV"
          )}
        </Button>
      )}

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
              className="h-full bg-workspace-accent transition-[width] duration-200 motion-reduce:transition-none"
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
              className="inline-flex min-h-11 items-center font-medium text-workspace-accent underline-offset-4 outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
            >
              View my applications
            </Link>
          )}
        </div>
      )}
    </div>
  );
}
