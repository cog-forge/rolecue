"use client";

import {
  AlertCircle,
  CheckCircle2,
  FileArchive,
  FileAudio,
  FileCode2,
  FileIcon,
  FileImage,
  FileSpreadsheet,
  FileText,
  FileVideo,
  LoaderCircle,
  RotateCcw,
  UploadCloud,
  X,
} from "lucide-react";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { useFormatter, useTranslations } from "next-intl";
import { useId, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export type FileUploadStatus = "queued" | "uploading" | "success" | "error";
export type FileUploadItem = {
  id: string;
  name: string;
  size?: number;
  type?: string;
  status?: FileUploadStatus;
  progress?: number;
  error?: string;
  file?: File;
  meta?: string;
  statusLabel?: string;
};

type FileUploadClassNames = Partial<
  Record<
    | "root"
    | "dropzone"
    | "queue"
    | "item"
    | "leading"
    | "content"
    | "name"
    | "meta"
    | "progress"
    | "action",
    string
  >
>;

export interface FileUploadProps {
  value?: FileUploadItem[];
  defaultValue?: FileUploadItem[];
  onValueChange?: (items: FileUploadItem[]) => void;
  onFilesAdded?: (items: FileUploadItem[], files: File[]) => void;
  onRemove?: (item: FileUploadItem) => void;
  onRetry?: (item: FileUploadItem) => void;
  validateSelection?: (files: File[]) => string | null;
  accept?: string;
  multiple?: boolean;
  maxFiles?: number;
  disabled?: boolean;
  variant?: "default" | "centered";
  title?: string;
  description?: string;
  browseLabel?: string;
  inputId?: string;
  inputLabel?: string;
  progressLabel?: string;
  removeLabel?: string;
  describedBy?: string;
  className?: string;
  classNames?: FileUploadClassNames;
}

const rowTransition = { duration: 0.18, ease: [0.16, 1, 0.3, 1] } as const;

const statusIcons = {
  queued: null,
  uploading: LoaderCircle,
  success: CheckCircle2,
  error: AlertCircle,
};

const statusTones = {
  queued: "text-muted-foreground",
  uploading: "text-foreground",
  success: "text-emerald-700 dark:text-emerald-400",
  error: "text-destructive",
};

function fileExtension(name: string) {
  const dot = name.lastIndexOf(".");
  return dot > 0 ? name.slice(dot + 1).toLowerCase() : "";
}

const fileIcons = {
  image: FileImage,
  video: FileVideo,
  audio: FileAudio,
  archive: FileArchive,
  spreadsheet: FileSpreadsheet,
  code: FileCode2,
  text: FileText,
  other: FileIcon,
};

function getFileKind({
  type = "",
  name,
}: FileUploadItem): keyof typeof fileIcons {
  const extension = fileExtension(name);

  if (type.startsWith("image/")) return "image";
  if (type.startsWith("video/")) return "video";
  if (type.startsWith("audio/")) return "audio";
  if (
    type.includes("zip") ||
    ["zip", "rar", "7z", "tar", "gz"].includes(extension)
  )
    return "archive";
  if (
    type.includes("spreadsheet") ||
    ["csv", "xls", "xlsx"].includes(extension)
  )
    return "spreadsheet";
  if (
    [
      "css",
      "html",
      "js",
      "jsx",
      "json",
      "mdx",
      "ts",
      "tsx",
      "xml",
      "yaml",
      "yml",
    ].includes(extension)
  )
    return "code";
  if (
    type.includes("pdf") ||
    type.startsWith("text/") ||
    ["pdf", "doc", "docx", "md", "txt"].includes(extension)
  )
    return "text";
  return "other";
}

let nextFileId = 0;

export function createFileUploadItem(file: File): FileUploadItem {
  return {
    id: `local-file-${nextFileId++}`,
    name: file.name,
    size: file.size,
    type: file.type,
    status: "queued",
    file,
  };
}

function FileUploadRow({
  item,
  disabled,
  progressLabel,
  removeLabel,
  onRemove,
  onRetry,
  classNames,
}: {
  item: FileUploadItem;
  disabled: boolean;
  progressLabel?: string;
  removeLabel?: string;
  onRemove?: (item: FileUploadItem) => void;
  onRetry?: (item: FileUploadItem) => void;
  classNames?: FileUploadClassNames;
}) {
  const t = useTranslations("common.fileUpload");
  const format = useFormatter();
  const reduceMotion = useReducedMotion();
  const status = item.status ?? "queued";
  const progress =
    status === "success"
      ? 100
      : typeof item.progress === "number" && Number.isFinite(item.progress)
        ? Math.max(0, Math.min(100, Math.round(item.progress)))
        : null;
  const showProgress = status === "uploading" && progress !== null;
  const FileTypeIcon = fileIcons[getFileKind(item)];
  const StatusIcon = statusIcons[status];
  const extension = fileExtension(item.name).toUpperCase();
  const size = item.size;
  const meta =
    item.meta ??
    [
      extension,
      size === undefined
        ? ""
        : size < 1024
          ? t("bytes", { size })
          : size < 1024 ** 2
            ? t("kilobytes", {
                size: format.number(size / 1024, { maximumFractionDigits: 1 }),
              })
            : t("megabytes", {
                size: format.number(size / 1024 ** 2, {
                  maximumFractionDigits: 1,
                }),
              }),
    ]
      .filter(Boolean)
      .join(" · ");
  const statusLabel = item.statusLabel ?? t(`status.${status}`);
  const actionClassName = cn(
    "size-11 text-muted-foreground motion-reduce:transition-none motion-reduce:active:translate-y-0",
    classNames?.action,
  );

  return (
    <motion.li
      initial={reduceMotion ? false : { opacity: 0, y: 4 }}
      animate={{ opacity: 1, y: 0 }}
      exit={reduceMotion ? undefined : { opacity: 0, y: -3 }}
      transition={reduceMotion ? { duration: 0 } : rowTransition}
      className={cn(
        "flex items-start gap-3 rounded-xl border border-border bg-background p-3",
        status === "error" && "border-destructive/35",
        classNames?.item,
      )}
    >
      <span
        className={cn(
          "grid size-10 shrink-0 place-items-center rounded-lg bg-muted text-foreground",
          classNames?.leading,
        )}
      >
        <FileTypeIcon className="size-5" aria-hidden="true" />
      </span>

      <div className={cn("min-w-0 flex-1 self-center", classNames?.content)}>
        <p
          className={cn(
            "text-sm font-medium leading-snug [overflow-wrap:anywhere]",
            classNames?.name,
          )}
        >
          {item.name}
        </p>
        {meta && (
          <p
            className={cn(
              "mt-1 text-xs text-muted-foreground",
              classNames?.meta,
            )}
          >
            {meta}
          </p>
        )}
        {statusLabel && (
          <p
            role={
              status === "uploading" || status === "success"
                ? "status"
                : undefined
            }
            className={cn(
              "mt-1.5 flex items-center gap-1.5 text-xs font-medium",
              statusTones[status],
            )}
          >
            {StatusIcon && (
              <StatusIcon
                aria-hidden="true"
                className={cn(
                  "size-3.5 shrink-0",
                  status === "uploading" && "motion-safe:animate-spin",
                )}
              />
            )}
            {statusLabel}
          </p>
        )}
        {status === "error" && item.error && (
          <p role="alert" className="mt-1 text-sm text-destructive">
            {item.error}
          </p>
        )}
        {showProgress && (
          <div
            role="progressbar"
            aria-label={progressLabel ?? t("progress", { name: item.name })}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={progress}
            aria-valuetext={`${progress}%`}
            className={cn(
              "mt-2.5 h-1.5 overflow-hidden rounded-full bg-muted",
              classNames?.progress,
            )}
          >
            <div
              className="h-full origin-left rounded-full bg-workspace-accent motion-safe:transition-transform motion-safe:duration-200"
              style={{ transform: `scaleX(${progress / 100})` }}
            />
          </div>
        )}
      </div>

      {((status === "error" && onRetry) || onRemove) && (
        <div className="flex shrink-0 gap-1">
          {status === "error" && onRetry && (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className={actionClassName}
              disabled={disabled}
              aria-label={t("retry", { name: item.name })}
              onClick={() => onRetry(item)}
            >
              <RotateCcw aria-hidden="true" />
            </Button>
          )}
          {onRemove && (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className={cn(actionClassName, "hover:text-destructive")}
              disabled={disabled}
              aria-label={removeLabel ?? t("remove", { name: item.name })}
              onClick={() => onRemove(item)}
            >
              <X aria-hidden="true" />
            </Button>
          )}
        </div>
      )}
    </motion.li>
  );
}

export function FileUpload({
  value,
  defaultValue,
  onValueChange,
  onFilesAdded,
  onRemove,
  onRetry,
  validateSelection,
  accept,
  multiple = true,
  maxFiles,
  disabled = false,
  variant = "default",
  title,
  description,
  browseLabel,
  inputId,
  inputLabel,
  progressLabel,
  removeLabel,
  describedBy,
  className,
  classNames,
}: FileUploadProps) {
  const t = useTranslations("common.fileUpload");
  const id = useId();
  const inputRef = useRef<HTMLInputElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const dragDepth = useRef(0);
  const [internalValue, setInternalValue] = useState(defaultValue ?? []);
  const [dragging, setDragging] = useState(false);
  const [selectionError, setSelectionError] = useState<string | null>(null);
  const items = value ?? internalValue;
  const maxReached = multiple
    ? maxFiles !== undefined && items.length >= maxFiles
    : maxFiles === 0;
  const unavailable = disabled || maxReached;
  const centered = variant === "centered";
  const hint = maxReached
    ? t("limitHint", { count: items.length, max: maxFiles ?? 0 })
    : description;
  const browse = browseLabel ?? t("browse");
  const hintId = hint ? `${id}-hint` : undefined;
  const errorId = selectionError ? `${id}-error` : undefined;
  const descriptionIds =
    [hintId, errorId, describedBy].filter(Boolean).join(" ") || undefined;

  function setItems(next: FileUploadItem[]) {
    if (value === undefined) setInternalValue(next);
    onValueChange?.(next);
  }

  function addFiles(incoming: File[]) {
    if (unavailable || !incoming.length) return;
    // The feature validates the same selection for both the picker and drag/drop.
    const error = validateSelection?.(incoming) ?? null;
    setSelectionError(error);
    if (error) return;

    const slots =
      maxFiles === undefined ? incoming.length : maxFiles - items.length;
    const files = incoming.slice(0, multiple ? slots : 1);
    const added = files.map(createFileUploadItem);
    setItems(multiple ? [...items, ...added] : added);
    onFilesAdded?.(added, files);
  }

  function removeItem(item: FileUploadItem) {
    setSelectionError(null);
    setItems(items.filter((entry) => entry.id !== item.id));
    onRemove?.(item);
    triggerRef.current?.focus();
  }

  function retryItem(item: FileUploadItem) {
    const retrying = {
      ...item,
      error: undefined,
      progress: undefined,
      status: "uploading" as const,
    };
    setItems(items.map((entry) => (entry.id === item.id ? retrying : entry)));
    onRetry?.(retrying);
  }

  return (
    <div
      className={cn(
        "@container/file-upload w-full min-w-0 space-y-2.5",
        className,
        classNames?.root,
      )}
    >
      <input
        ref={inputRef}
        id={inputId ?? id}
        type="file"
        aria-label={inputLabel}
        aria-hidden="true"
        accept={accept}
        multiple={multiple}
        disabled={unavailable}
        tabIndex={-1}
        className="sr-only"
        onChange={(event) => {
          addFiles(Array.from(event.currentTarget.files ?? []));
          event.currentTarget.value = "";
        }}
      />

      <div
        onDragEnter={(event) => {
          if (!event.dataTransfer.types.includes("Files")) return;
          event.preventDefault();
          dragDepth.current += 1;
          setDragging(!unavailable);
        }}
        onDragOver={(event) => {
          if (!event.dataTransfer.types.includes("Files")) return;
          event.preventDefault();
          event.dataTransfer.dropEffect = unavailable ? "none" : "copy";
        }}
        onDragLeave={() => {
          dragDepth.current = Math.max(0, dragDepth.current - 1);
          if (dragDepth.current === 0) setDragging(false);
        }}
        onDrop={(event) => {
          const files = Array.from(event.dataTransfer.files);
          if (files.length) event.preventDefault();
          dragDepth.current = 0;
          setDragging(false);
          addFiles(files);
        }}
      >
        <Button
          ref={triggerRef}
          type="button"
          variant="ghost"
          disabled={unavailable}
          aria-label={browse}
          aria-describedby={descriptionIds}
          aria-busy={items.some((item) => item.status === "uploading")}
          data-dragging={dragging && !unavailable}
          onClick={() => inputRef.current?.click()}
          className={cn(
            "group/upload relative h-auto w-full gap-3 rounded-xl border-dashed border-border bg-muted/30 p-4 whitespace-normal",
            "transition-colors hover:border-foreground/40 hover:bg-muted/65 focus-visible:ring-offset-2 focus-visible:ring-offset-background",
            "data-[dragging=true]:border-workspace-accent data-[dragging=true]:bg-workspace-accent/10",
            "motion-reduce:transition-none motion-reduce:active:translate-y-0",
            centered
              ? "min-h-40 flex-col py-5 text-center"
              : "flex-col items-start text-left @sm/file-upload:flex-row @sm/file-upload:items-center",
            classNames?.dropzone,
          )}
        >
          <span
            aria-hidden="true"
            className="grid size-11 shrink-0 place-items-center rounded-lg border border-border bg-background text-foreground motion-safe:transition-transform motion-safe:group-hover/upload:-translate-y-0.5 group-data-[dragging=true]/upload:text-workspace-accent"
          >
            <UploadCloud className="size-5" />
          </span>
          <span className={cn("min-w-0", centered ? "max-w-sm" : "flex-1")}>
            <span className="block text-sm font-semibold leading-snug text-foreground">
              {maxReached ? t("limitReached") : (title ?? t("dropHere"))}
            </span>
            {hint && (
              <span
                id={hintId}
                className="mt-1 block text-xs font-normal leading-relaxed text-muted-foreground"
              >
                {hint}
              </span>
            )}
          </span>
          <span className="inline-flex min-h-9 max-w-full items-center rounded-lg border border-border bg-background px-3 py-1.5 text-sm font-medium leading-snug text-foreground motion-safe:transition-colors group-hover/upload:border-foreground/25">
            {browse}
          </span>
        </Button>
      </div>
      {selectionError && (
        <p id={errorId} role="alert" className="text-sm text-destructive">
          {selectionError}
        </p>
      )}

      <ul
        role="list"
        className={cn("space-y-2 empty:hidden", classNames?.queue)}
      >
        <AnimatePresence initial={false}>
          {items.map((item) => (
            <FileUploadRow
              key={item.id}
              item={item}
              disabled={disabled}
              progressLabel={progressLabel}
              removeLabel={removeLabel}
              onRemove={
                onRemove || onValueChange || value === undefined
                  ? removeItem
                  : undefined
              }
              onRetry={onRetry ? retryItem : undefined}
              classNames={classNames}
            />
          ))}
        </AnimatePresence>
      </ul>
    </div>
  );
}
