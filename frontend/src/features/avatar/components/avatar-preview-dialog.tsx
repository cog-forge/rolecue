"use client";
import { ErrorState } from "@/components/ui/error-state";

import { useEffect, useMemo } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { isAxiosError } from "axios";
import { useTranslations } from "next-intl";
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
  const t = useTranslations("candidate.avatars.preview");
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
            {avatar?.name ?? t("defaultTitle")}
          </DialogTitle>
          <DialogDescription>{t("hint")}</DialogDescription>
        </DialogHeader>
        {missing ? (
          <p role="alert" className="text-sm">
            {t("missing")}
          </p>
        ) : detail.isError ? (
          <ErrorState
            description={t("openFailed")}
            onRetry={() => void refetch()}
            retryLabel={t("retry")}
          ></ErrorState>
        ) : (
          <div className="grid gap-6 sm:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
            <AvatarViewer
              access={access}
              label={t("viewerLabel", {
                name: avatar?.name ?? t("viewerFallback"),
              })}
              onAccessExpired={() => void refetch()}
            />
            <div className="min-w-0 space-y-5">
              {avatar ? (
                <dl className="space-y-4 text-sm">
                  <div>
                    <dt className="text-xs text-muted-foreground">
                      {t("personality")}
                    </dt>
                    <dd className="mt-1 leading-relaxed whitespace-pre-line [overflow-wrap:anywhere]">
                      {avatar.personality}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-xs text-muted-foreground">
                      {t("created")}
                    </dt>
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
                  <span className="sr-only">{t("loading")}</span>
                </div>
              )}
              {avatar && (
                <Button
                  variant="outline"
                  className="h-11 text-destructive hover:text-destructive"
                  onClick={() => onDelete(avatar)}
                >
                  <Trash2 aria-hidden /> {t("delete")}
                </Button>
              )}
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
