"use client";

import { useEffect } from "react";
import { useTranslations } from "next-intl";
import { UserAvatar } from "@/components/account/user-avatar";
import { Button } from "@/components/ui/button";
import { FileUpload } from "@/components/ui/file-upload";

const photoTypes = ["image/jpeg", "image/png", "image/webp"];
const maxBytes = 5 * 1024 * 1024;

// Undefined keeps the saved photo. Null removes it when the form is saved.
export type ProfilePhotoDraft =
  { file: File; previewUrl: string } | null | undefined;

export function ProfilePhotoField({
  userId,
  name,
  image,
  draft,
  onChange,
  disabled,
  progress,
  showCurrent = false,
  description,
}: {
  userId: string;
  name: string;
  image: string | null;
  draft: ProfilePhotoDraft;
  onChange: (draft: ProfilePhotoDraft) => void;
  disabled: boolean;
  progress: number | null;
  showCurrent?: boolean;
  description?: string;
}) {
  const t = useTranslations("common.profile.photo");
  const previewUrl = draft?.previewUrl;
  const keptImage = draft === null ? null : image;
  useEffect(() => {
    if (previewUrl) return () => URL.revokeObjectURL(previewUrl);
  }, [previewUrl]);

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-3">
        {(showCurrent || draft !== undefined) && (
          <UserAvatar
            id={userId}
            name={name}
            image={keptImage}
            previewUrl={previewUrl}
            className="size-12 rounded-full"
          />
        )}
        <div className="min-w-0">
          <p className="text-sm font-medium">{t("label")}</p>
          {description && (
            <p className="text-xs text-muted-foreground">{description}</p>
          )}
        </div>
      </div>
      <FileUpload
        value={
          draft
            ? [
                {
                  id: draft.previewUrl,
                  name: draft.file.name,
                  size: draft.file.size,
                  type: draft.file.type,
                  status: progress === null ? "queued" : "uploading",
                  progress: progress ?? undefined,
                  statusLabel:
                    progress === null
                      ? ""
                      : progress < 100
                        ? t("uploading", { progress })
                        : t("verifying"),
                },
              ]
            : []
        }
        accept={photoTypes.join(",")}
        multiple={false}
        disabled={disabled}
        title={t("dropHere")}
        description={t("formats")}
        browseLabel={draft || keptImage ? t("change") : t("choose")}
        inputLabel={t("fileInput")}
        removeLabel={t("discardFile")}
        progressLabel={t("progressLabel")}
        validateSelection={(files) => {
          if (files.length !== 1) return t("oneFile");
          const [file] = files;
          if (!photoTypes.includes(file.type) || file.size === 0)
            return t("invalidFormat");
          return file.size > maxBytes ? t("tooLarge") : null;
        }}
        onFilesAdded={(_, [file]) => {
          onChange({ file, previewUrl: URL.createObjectURL(file) });
        }}
        onRemove={() => onChange(undefined)}
      />
      {image && !draft && (
        <Button
          type="button"
          variant="ghost"
          size="sm"
          disabled={disabled}
          onClick={() => onChange(draft === null ? undefined : null)}
        >
          {draft === null ? t("restore") : t("remove")}
        </Button>
      )}
    </div>
  );
}
