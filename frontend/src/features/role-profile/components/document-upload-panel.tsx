"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { FileUpload } from "@/components/ui/file-upload";
import { useFormatters } from "@/lib/i18n/use-formatters";
import { pdfFileError } from "../lib/pdf-file";

export function DocumentUploadPanel({
  file,
  onChange,
  disabled,
  progress,
}: {
  file: File | null;
  onChange: (file: File | null) => void;
  disabled: boolean;
  progress: number | null;
}) {
  const t = useTranslations("candidate.roleProfiles.upload");
  const fmt = useFormatters();
  const [error, setError] = useState<string | null>(null);
  const select = ([selected]: File[]) => {
    if (!selected) return;
    const message = pdfFileError(selected, (key) => t(`errors.${key}`));
    setError(message);
    if (!message) onChange(selected);
  };

  return (
    <div className="space-y-2">
      <FileUpload
        variant="centered"
        value={
          file
            ? [
                {
                  id: file.name,
                  name: file.name,
                  type: file.type,
                  status: progress === null ? "queued" : "uploading",
                  progress: progress ?? undefined,
                  statusLabel:
                    progress === null
                      ? ""
                      : t("uploaded", { progress: String(progress) }),
                  meta: t("selected", {
                    size: fmt.number(file.size / 1024, {
                      minimumFractionDigits: 1,
                      maximumFractionDigits: 1,
                    }),
                  }),
                },
              ]
            : []
        }
        accept=".pdf,application/pdf"
        multiple={false}
        disabled={disabled}
        title={t("dropHere")}
        description={t("help")}
        browseLabel={file ? t("replace") : t("choose")}
        inputLabel={t("inputLabel")}
        removeLabel={t("remove")}
        progressLabel={t("progressLabel")}
        validateSelection={(files) => {
          if (files.length === 1) return null;
          setError(null);
          return t("errors.oneAtATime");
        }}
        onFilesAdded={(_, files) => select(files)}
        onRemove={() => {
          setError(null);
          onChange(null);
        }}
      />
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
    </div>
  );
}
