"use client";
import { ErrorState } from "@/components/ui/error-state";

import { useEffect, useMemo, useState } from "react";
import dynamic from "next/dynamic";
import { useTranslations } from "next-intl";
import { useQuery } from "@tanstack/react-query";
import { avatarKeys, getAvatar } from "@/features/avatar/api/inventory";

function PreviewLoading() {
  const t = useTranslations("recruiter.postings.preview");
  return <p role="status">{t("loading")}</p>;
}

const AvatarViewer = dynamic(
  () =>
    import("@/components/avatar-viewer/avatar-viewer").then(
      (module) => module.AvatarViewer,
    ),
  { ssr: false, loading: () => <PreviewLoading /> },
);

// Uses the same owner-scoped, short-lived model grant as Personal Avatar Studio.
export function InterviewerPreview({
  userId,
  avatarId,
}: {
  userId: string;
  avatarId: string;
}) {
  const t = useTranslations("recruiter.postings.preview");
  const [revealed, setRevealed] = useState(false);
  useEffect(() => {
    const frame = window.requestAnimationFrame(() => setRevealed(true));
    return () => window.cancelAnimationFrame(frame);
  }, []);
  const detail = useQuery({
    queryKey: avatarKeys.detail(userId, avatarId),
    queryFn: ({ signal }) => getAvatar(avatarId, signal),
    staleTime: 0,
    gcTime: 0,
    refetchOnMount: "always",
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

  return (
    <section
      aria-label={t("region")}
      data-open={revealed ? "true" : "false"}
      className="t-panel-slide grid items-start gap-5 rounded-xl border p-4 sm:grid-cols-[minmax(0,20rem)_1fr]"
    >
      {detail.isError ? (
        <ErrorState
          description={t("error")}
          onRetry={() => void detail.refetch()}
          retryLabel={t("retry")}
        ></ErrorState>
      ) : (
        <AvatarViewer
          key={avatarId}
          access={access}
          label={
            detail.data?.name
              ? t("viewerLabel", { name: detail.data.name })
              : t("viewerLabelFallback")
          }
          onAccessExpired={() => void detail.refetch()}
        />
      )}
      <div className="space-y-2 text-sm">
        <h3 className="font-medium">
          {detail.data?.name ?? t("fallbackName")}
        </h3>
        <p className="text-muted-foreground">{t("hint")}</p>
        {detail.data?.personality && (
          <p className="whitespace-pre-line text-muted-foreground">
            {detail.data.personality}
          </p>
        )}
      </div>
    </section>
  );
}
