"use client";

import { useEffect, useMemo } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { isAxiosError } from "axios";
import { Trash2 } from "lucide-react";
import { AvatarViewer } from "@/components/avatar-viewer/avatar-viewer";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { avatarKeys, getAvatar } from "../api/inventory";
import type { Avatar } from "../schemas/avatar-schema";
import { useFormatters } from "@/lib/i18n/use-formatters";

export function AvatarPreviewDialog({
  userId,
  avatarId,
  onClose,
  onDelete,
}: {
  userId: string;
  avatarId: string | null;
  onClose: () => void;
  onDelete: (avatar: Avatar) => void;
}) {
  const fmt = useFormatters();
  const cache = useQueryClient();
  const detail = useQuery({
    queryKey: avatarKeys.detail(userId, avatarId ?? ""),
    queryFn: ({ signal }) => getAvatar(avatarId ?? "", signal),
    enabled: avatarId !== null,
    // The presigned grant lives only in memory while the preview is open and is never reused later.
    gcTime: 0,
    staleTime: 0,
    refetchOnMount: "always",
    retry: false,
  });
  const missing =
    isAxiosError(detail.error) && detail.error.response?.status === 404;
  useEffect(() => {
    if (missing)
      void cache.invalidateQueries({ queryKey: avatarKeys.all(userId) });
  }, [missing, cache, userId]);

  const grant = detail.data?.model_access;
  // A stable object per grant: a new reference would restart the download.
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
  const avatar = detail.data;
  const { refetch } = detail;

  return (
    <Dialog
      open={avatarId !== null}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className="max-h-dvh overflow-y-auto max-sm:h-dvh max-sm:max-w-none max-sm:rounded-none max-sm:content-start sm:max-w-3xl sm:p-6">
        <DialogHeader className="pr-8">
          <DialogTitle className="text-lg [overflow-wrap:anywhere]">
            {avatar?.name ?? "Avatar preview"}
          </DialogTitle>
          <DialogDescription>
            Drag to turn the avatar, scroll or pinch to zoom.
          </DialogDescription>
        </DialogHeader>
        {missing ? (
          <p role="alert" className="text-sm">
            This avatar no longer exists in your inventory.
          </p>
        ) : detail.isError ? (
          <div role="alert" className="space-y-3">
            <p className="text-sm text-destructive">
              We couldn&apos;t open this avatar right now.
            </p>
            <Button
              variant="outline"
              className="h-11"
              onClick={() => void refetch()}
            >
              Try again
            </Button>
          </div>
        ) : (
          <div className="grid gap-6 sm:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
            <AvatarViewer
              access={access}
              label={`3D preview of ${avatar?.name ?? "your avatar"}`}
              onAccessExpired={() => void refetch()}
            />
            <div className="min-w-0 space-y-5">
              {avatar ? (
                <dl className="space-y-4 text-sm">
                  <div>
                    <dt className="text-xs text-muted-foreground">
                      Personality
                    </dt>
                    <dd className="mt-1 leading-relaxed whitespace-pre-line [overflow-wrap:anywhere]">
                      {avatar.personality}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-xs text-muted-foreground">Created</dt>
                    <dd className="mt-1">
                      <time dateTime={avatar.created_at}>
                        {fmt.date(avatar.created_at, "dayMonthYear")}
                      </time>
                    </dd>
                  </div>
                </dl>
              ) : (
                <div role="status" className="space-y-3">
                  <Skeleton className="h-4 w-24" />
                  <Skeleton className="h-16 w-full" />
                  <span className="sr-only">Loading avatar details…</span>
                </div>
              )}
              {avatar && (
                <Button
                  variant="outline"
                  className="h-11 text-destructive hover:text-destructive"
                  onClick={() => onDelete(avatar)}
                >
                  <Trash2 aria-hidden /> Delete avatar
                </Button>
              )}
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
