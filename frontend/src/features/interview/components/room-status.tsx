"use client";
import type { ReactNode } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { LoaderCircle, Play, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import type { PlaybackStatus } from "../playback/interviewer-playback";
import type { InterviewSessionView } from "../session/interview-session";
import { roomGlass } from "./room-control-button";

type EndCopy =
  "superseded" | "notEligible" | "denied" | "notFound" | "protocol" | "gaveUp";
const endCopy: Record<
  NonNullable<InterviewSessionView["end"]>["reason"],
  EndCopy | null
> = {
  completed: null,
  superseded: "superseded",
  "not-eligible": "notEligible",
  denied: "denied",
  "not-found": "notFound",
  protocol: "protocol",
  "gave-up": "gaveUp",
};

/** Connection, leaving, completion and end banners, plus the one-tap Play after an autoplay refusal.
 * A recruitment room passes `completion`: its recording upload replaces the report banner. */
export function RoomStatus({
  view,
  interviewId,
  playback,
  onPlay,
  onRetry,
  completion,
}: {
  view: InterviewSessionView;
  interviewId: string;
  playback: PlaybackStatus;
  onPlay: () => void;
  onRetry: () => void;
  completion?: ReactNode;
}) {
  const t = useTranslations("candidate.interviewRoom");
  const end = view.end ? endCopy[view.end.reason] : null;
  const reportHref = routes.reports.detail(interviewId);

  if (view.leaving === "submitting")
    return (
      <Banner role="status">
        <p className="flex items-center gap-2">
          <LoaderCircle
            aria-hidden
            className="size-4 shrink-0 motion-safe:animate-spin"
          />
          {t("status.submitting")}
        </p>
      </Banner>
    );
  if (view.phase === "COMPLETED" && completion) return completion;
  if (view.phase === "COMPLETED")
    return (
      <Banner role="status" title={t("status.completedTitle")}>
        <p>{t("status.completedText")}</p>
        <Button asChild variant="secondary">
          <Link href={reportHref}>{t("status.viewReport")}</Link>
        </Button>
      </Banner>
    );
  if (end)
    return (
      <Banner role="alert" title={t(`end.${end}Title`)}>
        <p>{t(`end.${end}Text`)}</p>
        <div className="flex flex-wrap gap-2">
          {(end === "superseded" || end === "gaveUp") && (
            <Button variant="secondary" onClick={onRetry}>
              <RotateCcw aria-hidden />
              {end === "superseded" ? t("end.continueHere") : t("end.retry")}
            </Button>
          )}
          <Button
            asChild
            variant="ghost"
            className="text-white hover:bg-white/10 hover:text-white"
          >
            <Link href={routes.history}>{t("end.history")}</Link>
          </Button>
        </div>
      </Banner>
    );
  if (view.leaving === "failed")
    return (
      <Banner role="alert" title={t("status.leaveFailedTitle")}>
        <p>{t("status.leaveFailedText")}</p>
      </Banner>
    );
  if (playback === "blocked" && view.phase === "INTERVIEWER_SPEAKING")
    return (
      <Banner role="alert">
        <p>{t("playback.blocked")}</p>
        <Button variant="secondary" onClick={onPlay}>
          <Play aria-hidden />
          {t("playback.play")}
        </Button>
      </Banner>
    );
  const progress =
    view.phase === "CONNECTING"
      ? t("status.connecting")
      : view.phase === "RECONNECTING"
        ? t("status.reconnecting")
        : view.phase === "ASSET_LOADING"
          ? t("status.loading")
          : view.phase === "TRANSITIONING"
            ? t("status.transitioning")
            : null;
  if (!progress) return null;
  return (
    <Banner role="status">
      <p className="flex items-center gap-2">
        <LoaderCircle
          aria-hidden
          className="size-4 shrink-0 motion-safe:animate-spin"
        />
        {progress}
      </p>
    </Banner>
  );
}

export function Banner({
  role,
  title,
  children,
}: {
  role: "status" | "alert";
  title?: string;
  children: ReactNode;
}) {
  return (
    <div
      role={role}
      className={`w-full space-y-3 rounded-2xl p-4 text-sm leading-relaxed ${roomGlass}`}
    >
      {title && <h2 className="font-medium">{title}</h2>}
      {children}
    </div>
  );
}
