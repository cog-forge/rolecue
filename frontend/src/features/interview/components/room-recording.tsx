"use client";
import { useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { LoaderCircle, RotateCcw, Video } from "lucide-react";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { useFormatters } from "@/lib/i18n/use-formatters";
import type { UploadSnapshot } from "../recording/recording-upload";
import type { SessionRecorderSnapshot } from "../recording/session-recorder";
import { RecordingDisclosure } from "./recording-disclosure";
import { Banner } from "./room-status";

const ghost = "text-white hover:bg-white/10 hover:text-white";

/** Before a recruitment interview starts: the disclosure, the camera request and its recovery. */
export function RecordingGate({
  recorder,
  onRetry,
}: {
  recorder: SessionRecorderSnapshot;
  onRetry: () => void;
}) {
  const t = useTranslations("candidate.interviewRoom.recording");
  const error = recorder.status === "error" ? recorder.error : null;
  return (
    <Banner role={error ? "alert" : "status"} title={t("gate.title")}>
      <p>{t("gate.text")}</p>
      <RecordingDisclosure />
      {error ? (
        <>
          <p>{t(`error.${error}`)}</p>
          <Button variant="secondary" onClick={onRetry}>
            <RotateCcw aria-hidden />
            {t("gate.retry")}
          </Button>
        </>
      ) : (
        <p className="flex items-center gap-2">
          <LoaderCircle
            aria-hidden
            className="size-4 shrink-0 motion-safe:animate-spin"
          />
          {t("gate.requesting")}
        </p>
      )}
    </Banner>
  );
}

/** During the interview: the recording stopped (device lost) or reached its size limit. */
export function RecordingNotice({
  recorder,
  onResume,
}: {
  recorder: SessionRecorderSnapshot;
  onResume: () => void;
}) {
  const t = useTranslations("candidate.interviewRoom.recording");
  if (recorder.limitReached)
    return (
      <Banner role="status">
        <p>{t("limit")}</p>
      </Banner>
    );
  if (recorder.status === "requesting")
    return (
      <Banner role="status">
        <p className="flex items-center gap-2">
          <LoaderCircle
            aria-hidden
            className="size-4 shrink-0 motion-safe:animate-spin"
          />
          {t("gate.requesting")}
        </p>
      </Banner>
    );
  const error = recorder.status === "error" ? recorder.error : null;
  if (!error) return null;
  return (
    <Banner role="alert" title={t("interrupted.title")}>
      <p>{t(`error.${error}`)}</p>
      <p className="text-white/75">{t("interrupted.note")}</p>
      <Button variant="secondary" onClick={onResume}>
        <Video aria-hidden />
        {t("interrupted.resume")}
      </Button>
    </Banner>
  );
}

/** A small mark in the room header while the camera and microphone are recorded. */
export function RecordingIndicator({ recording }: { recording: boolean }) {
  const t = useTranslations("candidate.interviewRoom.recording");
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full border border-white/15 px-2 py-0.5 text-[10px] text-white/80">
      <span
        aria-hidden
        className={`size-1.5 rounded-full ${recording ? "bg-red-500 motion-safe:animate-pulse" : "bg-white/40"}`}
      />
      {recording ? t("indicator") : t("indicatorStopped")}
    </span>
  );
}

/** After a recruitment interview: the upload, its retries, and what the Candidate can do next. */
export function RecordingUploadPanel({
  upload,
  interviewId,
  onRetry,
  onGiveUp,
}: {
  upload: UploadSnapshot;
  interviewId: string;
  onRetry: () => void;
  onGiveUp: () => void;
}) {
  const t = useTranslations("candidate.interviewRoom.recording.upload");
  const fmt = useFormatters();
  const [confirming, setConfirming] = useState(false);
  const busy =
    upload.status === "idle" ||
    upload.status === "checking" ||
    upload.status === "preparing" ||
    upload.status === "uploading" ||
    upload.status === "retrying";
  const failed = upload.status === "failed";
  const message =
    upload.status === "uploading"
      ? t("uploading", { percent: fmt.percent(upload.progress * 100, 0) })
      : upload.status === "idle"
        ? t("checking")
        : failed
          ? upload.abandoned
            ? t("abandoned")
            : upload.reason === "rejected"
              ? t("rejected")
              : upload.reason === "exhausted"
                ? t("exhausted")
                : t("failed")
          : t(upload.status);
  const recoverable =
    failed && !upload.abandoned && upload.reason === "network";
  return (
    <Banner role={failed ? "alert" : "status"} title={t("title")}>
      <p className="flex items-center gap-2">
        {busy && (
          <LoaderCircle
            aria-hidden
            className="size-4 shrink-0 motion-safe:animate-spin"
          />
        )}
        {message}
      </p>
      {upload.status === "uploading" && (
        <progress
          aria-label={t("progressLabel")}
          max={1}
          value={upload.progress}
          className="block h-1.5 w-full overflow-hidden rounded-full accent-white"
        />
      )}
      {upload.partial && <p className="text-white/75">{t("partial")}</p>}
      {busy && <p className="text-white/75">{t("keepOpen")}</p>}
      {recoverable && <p className="text-white/75">{t("failedNote")}</p>}
      {recoverable && !confirming && (
        <div className="flex flex-wrap gap-2">
          <Button variant="secondary" onClick={onRetry}>
            <RotateCcw aria-hidden />
            {t("retry")}
          </Button>
          <Button
            variant="ghost"
            className={ghost}
            onClick={() => setConfirming(true)}
          >
            {t("giveUp")}
          </Button>
        </div>
      )}
      {recoverable && confirming && (
        <div
          role="group"
          aria-labelledby={`${interviewId}-give-up`}
          className="space-y-2 rounded-xl border border-white/15 p-3"
        >
          <p id={`${interviewId}-give-up`} className="font-medium">
            {t("confirmTitle")}
          </p>
          <p className="text-white/75">{t("confirmText")}</p>
          <div className="flex flex-wrap gap-2">
            <Button
              variant="destructive"
              onClick={() => {
                setConfirming(false);
                onGiveUp();
              }}
            >
              {t("confirm")}
            </Button>
            <Button
              variant="ghost"
              className={ghost}
              onClick={() => setConfirming(false)}
            >
              {t("keep")}
            </Button>
          </div>
        </div>
      )}
      {upload.status === "ready" ? (
        <p className="text-white/75">{t("openingReport")}</p>
      ) : (
        !busy &&
        !recoverable && (
          <Button asChild variant="secondary">
            <Link href={routes.reports.detail(interviewId)}>
              {t("viewReport")}
            </Link>
          </Button>
        )
      )}
    </Banner>
  );
}
