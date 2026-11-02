"use client";

import { useRef, useState } from "react";
import { FileText, Upload, X } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { pdfFileError } from "../api/contract";
import { useFormatters } from "@/lib/i18n/use-formatters";

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
  const input = useRef<HTMLInputElement>(null);
  const [error, setError] = useState<string | null>(null);
  const select = (selected: File | undefined) => {
    if (!selected) return;
    const message = pdfFileError(selected, (key) => t(`errors.${key}`));
    setError(message);
    if (!message) onChange(selected);
    if (input.current) input.current.value = "";
  };
  return (
    <div
      aria-describedby="document-help"
      className="rounded-xl border border-dashed bg-muted/30 px-5 py-7 text-center"
      onDragOver={(event) => event.preventDefault()}
      onDrop={(event) => {
        event.preventDefault();
        if (!disabled) {
          if (event.dataTransfer.files.length !== 1)
            setError(t("errors.oneAtATime"));
          else select(event.dataTransfer.files[0]);
        }
      }}
    >
      <div className="flex flex-col items-center gap-3">
        {file ? (
          <FileText
            className="size-6 text-workspace-accent"
            aria-hidden="true"
          />
        ) : (
          <Upload className="size-6 text-muted-foreground" aria-hidden="true" />
        )}
        <p className="max-w-full break-words text-sm font-medium">
          {file ? file.name : t("dropHere")}
        </p>
        <p
          id="document-help"
          className="max-w-sm text-xs leading-relaxed text-muted-foreground"
        >
          {file
            ? t("selected", {
                size: fmt.number(file.size / 1024, {
                  minimumFractionDigits: 1,
                  maximumFractionDigits: 1,
                }),
              })
            : t("help")}
        </p>
        <Input
          ref={input}
          type="file"
          accept=".pdf,application/pdf"
          className="sr-only"
          tabIndex={-1}
          aria-label={t("inputLabel")}
          disabled={disabled}
          onChange={(event) => select(event.target.files?.[0])}
        />
        <div className="flex flex-wrap justify-center gap-2">
          <Button
            type="button"
            variant="outline"
            disabled={disabled}
            className="h-auto min-h-11 py-2 whitespace-normal"
            onClick={() => input.current?.click()}
          >
            {file ? t("replace") : t("choose")}
          </Button>
          {file && (
            <Button
              type="button"
              variant="ghost"
              disabled={disabled}
              className="h-auto min-h-11 py-2 whitespace-normal hover:text-destructive"
              onClick={() => {
                setError(null);
                onChange(null);
              }}
            >
              <X aria-hidden="true" /> {t("remove")}
            </Button>
          )}
        </div>
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
        {progress !== null && (
          <div className="w-full max-w-sm space-y-2 text-left">
            <progress
              className="h-2 w-full accent-workspace-accent"
              max={100}
              value={progress}
              aria-label={t("progressLabel")}
            />
            <p className="text-xs text-muted-foreground">
              {t("uploaded", { progress: String(progress) })}
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
