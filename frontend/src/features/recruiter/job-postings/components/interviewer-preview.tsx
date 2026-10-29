"use client";

import { useEffect, useMemo, useState } from "react";
import dynamic from "next/dynamic";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { avatarKeys, getAvatar } from "@/features/avatar/api/inventory";

const AvatarViewer = dynamic(
  () =>
    import("@/components/avatar-viewer/avatar-viewer").then(
      (module) => module.AvatarViewer,
    ),
  { ssr: false, loading: () => <p role="status">Loading 3D preview…</p> },
);

// Uses the same owner-scoped, short-lived model grant as Personal Avatar Studio.
export function InterviewerPreview({
  userId,
  avatarId,
}: {
  userId: string;
  avatarId: string;
}) {
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
      aria-label="Selected interviewer preview"
      data-open={revealed ? "true" : "false"}
      className="t-panel-slide grid items-start gap-5 rounded-xl border p-4 sm:grid-cols-[minmax(0,20rem)_1fr]"
    >
      {detail.isError ? (
        <div role="alert" className="space-y-3 text-sm">
          <p>
            We couldn&apos;t load this interviewer preview. Your selection is
            still saved in the form.
          </p>
          <Button
            type="button"
            variant="outline"
            onClick={() => void detail.refetch()}
          >
            Retry preview
          </Button>
        </div>
      ) : (
        <AvatarViewer
          key={avatarId}
          access={access}
          label={`3D preview of ${detail.data?.name ?? "selected interviewer"}`}
          onAccessExpired={() => void detail.refetch()}
        />
      )}
      <div className="space-y-2 text-sm">
        <h3 className="font-medium">
          {detail.data?.name ?? "Selected interviewer"}
        </h3>
        <p className="text-muted-foreground">
          Drag to rotate. Scroll or pinch to zoom. Every candidate will use this
          interviewer and the voice selected above.
        </p>
        {detail.data?.personality && (
          <p className="whitespace-pre-line text-muted-foreground">
            {detail.data.personality}
          </p>
        )}
      </div>
    </section>
  );
}
