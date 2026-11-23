"use client";

import { useState } from "react";
import Link from "next/link";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { FileUpload } from "@/components/ui/file-upload";
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
  type CreatedApplication,
} from "../schemas/candidate-application-schema";
import { applyErrorMessage, canRetryApply } from "../utils/api-error";
import type { Formatters } from "@/lib/i18n/formatters";
import { useFormatters } from "@/lib/i18n/use-formatters";
import { useTranslations } from "next-intl";

const isPdf = (file: File) =>
  file.type === "application/pdf" || file.name.toLowerCase().endsWith(".pdf");

// Which message to show; it is translated when rendered so a language switch reaches it.
type CvFileError = "notPdf" | "empty" | "tooLarge";

function cvFileError(file: File): CvFileError | null {
  if (!isPdf(file)) return "notPdf";
  if (file.size === 0) return "empty";
  if (file.size > CV_MAX_BYTES) return "tooLarge";
  return null;
}

type CvFormTranslator = ReturnType<
  typeof useTranslations<"candidate.applications.cvForm">
>;

const fileSize = (bytes: number, fmt: Formatters, t: CvFormTranslator) =>
  bytes < 1024 * 1024
    ? t("sizeKb", {
        size: fmt.number(bytes / 1024, { maximumFractionDigits: 1 }),
      })
    : t("sizeMb", {
        size: fmt.number(bytes / 1024 / 1024, { maximumFractionDigits: 1 }),
      });

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
  const applyT = useTranslations("candidate.applications.applyErrors");
  const t = useTranslations("candidate.applications.cvForm");
  const user = useSessionUser();
  const cache = useQueryClient();
  const [file, setFile] = useState<File | null>(null);
  const [fileError, setFileError] = useState<CvFileError | null>(null);
  const [progress, setProgress] = useState(0);

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
  const message = failed
    ? applyErrorMessage(upload.error, errorsT, applyT)
    : fileError
      ? t(`errors.${fileError}`)
      : null;

  const pick = (picked: File | null) => {
    upload.reset();
    const error = picked ? cvFileError(picked) : null;
    setFile(error ? null : picked);
    setFileError(error);
  };

  return (
    <div className="space-y-4">
      <FileUpload
        inputId="cv-file"
        value={
          file
            ? [
                {
                  id: file.name,
                  name: file.name,
                  type: file.type,
                  meta: fileSize(file.size, fmt, t),
                  status: busy ? "uploading" : failed ? "error" : "queued",
                  progress,
                  statusLabel: busy
                    ? progress < 100
                      ? t("uploading", { progress })
                      : t("saving")
                    : undefined,
                },
              ]
            : []
        }
        accept={CV_FILE_TYPES}
        multiple={false}
        disabled={busy}
        title={t("dropHere")}
        description={t("hint")}
        browseLabel={file ? t("chooseAnother") : t("choose")}
        progressLabel={t("progressLabel")}
        onFilesAdded={(_, [selected]) => pick(selected ?? null)}
        onRemove={() => pick(null)}
      />

      {message && (
        <div role="alert" className="space-y-2 text-sm text-destructive">
          <p>{message}</p>
          {failed && !retryable && (
            <Link
              href={routes.applications}
              className="inline-flex min-h-11 items-center font-medium text-foreground underline underline-offset-4 outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
            >
              {t("viewMine")}
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
            {t("tryAgain")}
          </>
        ) : busy ? (
          t("sending")
        ) : (
          t("send")
        )}
      </Button>
    </div>
  );
}
