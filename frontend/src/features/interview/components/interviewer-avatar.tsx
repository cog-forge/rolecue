"use client";

import { memo, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { AvatarViewer } from "@/components/avatar-viewer/avatar-viewer";
import { avatarKeys, getAvatar } from "@/features/avatar/api/inventory";
import type { RoomDetails } from "../api/room-details";

export const InterviewerAvatar = memo(function InterviewerAvatar({
  userId,
  session,
  cameraView,
  atmosphere,
  viewResetSignal,
  onReadyChange,
  backgroundSrc,
  horizontalOffset,
}: {
  userId: string;
  session: RoomDetails;
  cameraView: "conversation" | "close";
  atmosphere: "warm" | "daylight" | "evening";
  viewResetSignal: number;
  onReadyChange: (ready: boolean) => void;
  backgroundSrc: string;
  horizontalOffset: number;
}) {
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
  const grant = detail.data?.model_access;
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
      ? "The company interviewer is shown in 2D for this interview."
      : !avatarId
        ? "No interviewer avatar was selected for this room."
        : detail.isError
          ? "We couldn’t load the interviewer avatar."
          : undefined;

  return (
    <AvatarViewer
      access={fallbackReason ? null : access}
      label={
        detail.data?.name
          ? `Interviewer: ${detail.data.name}`
          : "Your interviewer"
      }
      onAccessExpired={() => {
        void detail.refetch();
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
