"use client";

import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import Image from "next/image";
import { useQuery } from "@tanstack/react-query";
import { UserRound } from "lucide-react";
import { renderAvatarPortrait } from "@/components/avatar-viewer/render-portrait";
import { StudioBackdrop } from "@/components/avatar-viewer/studio-backdrop";
import { isWebGLAvailable } from "@/components/avatar-viewer/webgl";
import { avatarPortraitKey, getAvatar } from "@/features/avatar/api/inventory";
import type { Avatar } from "@/features/avatar/schemas/avatar-schema";
import { cn } from "@/lib/utils";

const noSubscription = () => () => {};

// A still portrait, never a live 3D canvas: setup only needs to recognise the avatar. It shares the
// Avatar Studio's portrait cache, so an avatar drawn there appears here at once.
export function AvatarPortrait({
  avatar,
  userId,
  compact = false,
  className,
}: {
  avatar: Avatar;
  userId: string;
  /** A thumbnail too small for words: the fallback shows the silhouette icon only. */
  compact?: boolean;
  className?: string;
}) {
  const frame = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const webgl = useSyncExternalStore(
    noSubscription,
    isWebGLAvailable,
    () => null,
  );

  // Each portrait downloads a whole model, so only render the ones on (or near) the screen.
  useEffect(() => {
    const element = frame.current;
    if (!element) return;
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry?.isIntersecting) {
          setVisible(true);
          observer.disconnect();
        }
      },
      { rootMargin: "200px" },
    );
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  const portrait = useQuery({
    queryKey: avatarPortraitKey(userId, avatar.id, avatar.updated_at),
    queryFn: async ({ signal }) => {
      const detail = await getAvatar(avatar.id, signal);
      return renderAvatarPortrait(
        {
          url: detail.model_access.url,
          method: detail.model_access.method,
          headers: detail.model_access.headers,
          expiresAt: detail.model_access.expires_at,
        },
        signal,
      );
    },
    enabled: visible && webgl === true,
    staleTime: Infinity,
    gcTime: Infinity,
    retry: false,
  });
  const failed = webgl === false || portrait.isError;

  return (
    <div
      ref={frame}
      className={cn("relative aspect-[4/5] overflow-hidden", className)}
    >
      <StudioBackdrop portrait />
      {failed ? (
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 bg-background/85 p-3 text-center">
          <UserRound
            aria-hidden
            className={cn(
              "text-muted-foreground",
              compact ? "size-5" : "size-9",
            )}
            strokeWidth={compact ? 1.5 : 1.25}
          />
          {!compact && (
            <p className="text-xs text-muted-foreground">Preview unavailable</p>
          )}
        </div>
      ) : (
        <div
          className={cn(
            "t-skel h-full w-full",
            portrait.data && loaded && "is-revealed",
          )}
        >
          <div className="t-skel-skeleton is-pulsing [--pulse-count:infinite]">
            {/* A head-and-shoulders silhouette at the portrait's real size. */}
            <div className="absolute top-[18%] left-1/2 size-[30%] -translate-x-1/2 rounded-full bg-foreground/8" />
            <div className="absolute inset-x-[16%] top-[50%] bottom-0 rounded-t-[45%] bg-foreground/8" />
          </div>
          <div className="t-skel-content">
            {portrait.data && (
              // An in-memory render (blob: URL), so there is nothing for the optimiser to fetch.
              <Image
                src={portrait.data}
                alt=""
                fill
                unoptimized
                sizes="(min-width: 1024px) 18vw, (min-width: 640px) 30vw, 45vw"
                className="object-cover"
                onLoad={() => setLoaded(true)}
              />
            )}
          </div>
        </div>
      )}
    </div>
  );
}
