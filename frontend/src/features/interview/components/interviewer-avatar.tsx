"use client";

import { useTranslations } from "next-intl";
import {
  companyInterviewerKey,
  getCompanyInterviewer,
} from "@/features/candidate/applications/api/company-interviewer";
import { memo, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { AvatarViewer } from "@/components/avatar-viewer/avatar-viewer";
import { avatarKeys, getAvatar } from "@/features/avatar/api/inventory";
import type {
  AvatarPerformance,
  AvatarDirector,
} from "@/components/avatar-viewer/avatar-director";
import type { RoomDetails } from "../api/room-details";

export const InterviewerAvatar = memo(function InterviewerAvatar({
  performance,
  onDirectorReady,
  userId,
  session,
  cameraView,
  atmosphere,
  viewResetSignal,
  onReadyChange,
  backgroundSrc,
  horizontalOffset,
}: {
  performance?: AvatarPerformance;
  onDirectorReady?: (director: AvatarDirector | null) => void;
  userId: string;
  session: RoomDetails;
  cameraView: "conversation" | "close";
  atmosphere: "warm" | "daylight" | "evening";
  viewResetSignal: number;
  onReadyChange: (ready: boolean) => void;
  backgroundSrc: string;
  horizontalOffset: number;
}) {
  const t = useTranslations("candidate.applications.interviewer");
  const company = useQuery({
    queryKey: companyInterviewerKey(userId, session.id, true),
    queryFn: ({ signal }) => getCompanyInterviewer(session.id, signal, true),
    enabled: session.type === "recruitment",
    staleTime: 0,
    gcTime: 0,
    retry: false,
    refetchOnMount: "always",
    refetchOnWindowFocus: false,
  });
  const avatarId = session.type === "practice" ? session.avatar_id : null;
  const detail = useQuery({
    queryKey: avatarKeys.detail(userId, avatarId ?? ""),
    queryFn: ({ signal }) => getAvatar(avatarId ?? "", signal),
    enabled: avatarId !== null,
    staleTime: 0,
    gcTime: 0,
    refetchOnMount: "always",
    refetchOnWindowFocus: false,
    retry: false,
  });
  const grant =
    session.type === "recruitment"
      ? company.data?.model_access
      : detail.data?.model_access;
  const access = useMemo(
    () =>
      grant
        ? {
            url: grant.url,
            method: grant.method,
            headers: grant.headers,
            expiresAt: grant.expires_at,
          }
        : null,
    [grant],
  );
  const fallbackReason =
    session.type === "recruitment"
      ? company.isError
        ? t("roomFallback")
        : undefined
      : !avatarId
        ? "No interviewer avatar was selected for this room."
        : detail.isError
          ? "We couldn’t load the interviewer avatar."
          : undefined;

  return (
    <AvatarViewer
      performance={performance}
      onDirectorReady={onDirectorReady}
      access={fallbackReason ? null : access}
      label={
        session.type === "recruitment"
          ? (company.data?.name ?? t("roomLabel"))
          : detail.data?.name
            ? `Interviewer: ${detail.data.name}`
            : "Your interviewer"
      }
      onAccessExpired={() => {
        if (session.type === "recruitment") void company.refetch();
        else void detail.refetch();
      }}
      fallbackReason={fallbackReason}
      backgroundSrc={backgroundSrc}
      horizontalOffset={horizontalOffset}
      presentation="interview"
      cameraView={cameraView}
      atmosphere={atmosphere}
      viewResetSignal={viewResetSignal}
      onReadyChange={onReadyChange}
    />
  );
});
