"use client";

import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import Image from "next/image";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import {
  Coins,
  Plus,
  Rotate3d,
  RotateCcw,
  Trash2,
  UserRound,
} from "lucide-react";
import { renderAvatarPortrait } from "@/components/avatar-viewer/render-portrait";
import { StudioBackdrop } from "@/components/avatar-viewer/studio-backdrop";
import { isWebGLAvailable } from "@/components/avatar-viewer/webgl";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { cn } from "@/lib/utils";
import { avatarPortraitKey, getAvatar } from "../api/inventory";
import { slotPriceCoins, type Avatar } from "../schemas/avatar-schema";
import { useFormatters } from "@/lib/i18n/use-formatters";

const noSubscription = () => () => {};

// Cards enter one after another; the delay stops growing after a row so a full page stays quick.
export function slotEntrance(index: number) {
  return {
    className:
      "animate-in fade-in slide-in-from-bottom-2 fill-mode-both duration-(--duration-slow) ease-(--ease-smooth-out) motion-reduce:animate-none",
    style: { animationDelay: `${Math.min(index, 8) * 40}ms` },
  };
}

const cardClass =
  "group relative flex min-w-0 flex-col overflow-hidden rounded-2xl transition-[translate,border-color] duration-(--duration-fast) ease-(--ease-smooth-out) motion-reduce:transition-none";

export function AvatarSlotCard({
  avatar,
  userId,
  index,
  onOpen,
  onDelete,
}: {
  avatar: Avatar;
  userId: string;
  index: number;
  onOpen: () => void;
  onDelete: (trigger: HTMLButtonElement) => void;
}) {
  const fmt = useFormatters();
  const card = useRef<HTMLLIElement>(null);
  const [visible, setVisible] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const webgl = useSyncExternalStore(
    noSubscription,
    isWebGLAvailable,
    () => null,
  );

  // Render only cards that are on (or near) the screen; each portrait downloads a whole model.
  useEffect(() => {
    const element = card.current;
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
      // A fresh five-minute grant per render; the portrait outlives it as an in-memory image.
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
  const entrance = slotEntrance(index);

  return (
    <li
      ref={card}
      className={cn(
        cardClass,
        "border bg-card hover:-translate-y-0.5 hover:border-foreground/25 motion-reduce:hover:translate-y-0",
        entrance.className,
      )}
      style={entrance.style}
    >
      <div className="relative aspect-[4/5] overflow-hidden">
        <StudioBackdrop portrait />
        {failed ? (
          <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 bg-background/90 p-4 text-center">
            <UserRound
              aria-hidden
              className="size-10 text-muted-foreground"
              strokeWidth={1.25}
            />
            <p className="text-xs text-muted-foreground">
              {webgl === false
                ? "3D isn't available on this device"
                : "Couldn't render the preview"}
            </p>
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
                  sizes="(min-width: 1024px) 25vw, (min-width: 640px) 33vw, 50vw"
                  className="object-cover"
                  onLoad={() => setLoaded(true)}
                />
              )}
            </div>
          </div>
        )}
        <button
          type="button"
          onClick={onOpen}
          aria-label={`Open ${avatar.name} in 3D`}
          className="absolute inset-0 z-10 rounded-t-2xl outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset"
        >
          <span className="absolute right-2.5 bottom-2.5 flex items-center gap-1 rounded-full bg-background/90 px-2.5 py-1 text-xs font-medium text-foreground">
            <Rotate3d aria-hidden className="size-3.5" />
            3D
          </span>
        </button>
        {portrait.isError && (
          <Button
            type="button"
            variant="secondary"
            size="sm"
            className="absolute bottom-2.5 left-2.5 z-20 h-8"
            onClick={() => void portrait.refetch()}
          >
            <RotateCcw aria-hidden /> Retry
          </Button>
        )}
      </div>
      <div className="flex flex-1 flex-col gap-2 p-3.5 pr-2">
        <div className="flex items-start gap-1">
          <div className="min-w-0 flex-1">
            <h2 className="truncate text-sm font-semibold" title={avatar.name}>
              {avatar.name}
            </h2>
            <p className="text-xs text-muted-foreground">
              <time dateTime={avatar.created_at}>
                {fmt.date(avatar.created_at, "dayMonthYear")}
              </time>
            </p>
          </div>
          <Button
            variant="ghost"
            size="icon"
            className="-mt-1.5 size-11 shrink-0 text-muted-foreground hover:text-destructive"
            aria-label={`Delete ${avatar.name}`}
            onClick={(event) => onDelete(event.currentTarget)}
          >
            <Trash2 aria-hidden />
          </Button>
        </div>
        <p className="line-clamp-2 pr-1.5 text-xs leading-relaxed text-muted-foreground [overflow-wrap:anywhere]">
          {avatar.personality}
        </p>
      </div>
    </li>
  );
}

const quietCardClass = "border border-dashed bg-muted/20";

function SlotPortraitPlaceholder({ children }: { children: React.ReactNode }) {
  return (
    <div className="relative grid aspect-[4/5] place-items-center">
      {children}
    </div>
  );
}

export function EmptySlotCard({
  slot,
  capacity,
  index,
  first,
}: {
  slot: number;
  capacity: number;
  index: number;
  first: boolean;
}) {
  const entrance = slotEntrance(index);
  return (
    <li
      className={cn(
        cardClass,
        quietCardClass,
        "hover:-translate-y-0.5 hover:border-foreground/30 motion-reduce:hover:translate-y-0",
        entrance.className,
      )}
      style={entrance.style}
    >
      <Link
        href={routes.avatarStudioNew}
        aria-label={`Create an avatar in empty slot ${slot} of ${capacity}`}
        className="flex h-full flex-col rounded-2xl outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <SlotPortraitPlaceholder>
          {first ? (
            <span
              aria-hidden
              className="flex size-16 items-center justify-center rounded-full bg-foreground text-background transition-transform duration-(--duration-fast) ease-(--ease-smooth-out) group-hover:scale-105 motion-reduce:transition-none motion-reduce:group-hover:scale-100"
            >
              <Plus className="size-6" strokeWidth={1.75} />
            </span>
          ) : (
            <Plus
              aria-hidden
              className="size-6 text-muted-foreground/60 transition-colors duration-(--duration-fast) group-hover:text-foreground motion-reduce:transition-none"
              strokeWidth={1.5}
            />
          )}
        </SlotPortraitPlaceholder>
        <div className="p-3.5">
          <p
            className={cn(
              "text-sm",
              first ? "font-medium" : "text-muted-foreground",
            )}
          >
            {first ? "Create an avatar" : "Empty slot"}
          </p>
          <p className="text-xs text-muted-foreground">
            Slot {slot} of {capacity}
          </p>
        </div>
      </Link>
    </li>
  );
}

export function AddSlotsCard({
  index,
  onBuy,
}: {
  index: number;
  onBuy: () => void;
}) {
  const entrance = slotEntrance(index);
  return (
    <li
      className={cn(
        cardClass,
        quietCardClass,
        "hover:-translate-y-0.5 hover:border-foreground/30 motion-reduce:hover:translate-y-0",
        entrance.className,
      )}
      style={entrance.style}
    >
      <button
        type="button"
        onClick={onBuy}
        className="flex h-full flex-col rounded-2xl text-left outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <SlotPortraitPlaceholder>
          <span
            aria-hidden
            className="flex size-16 items-center justify-center rounded-full border border-dashed border-foreground/30"
          >
            <Coins className="size-6" strokeWidth={1.5} />
          </span>
        </SlotPortraitPlaceholder>
        <div className="p-3.5">
          <p className="text-sm font-medium">Add more slots</p>
          <p className="text-xs text-muted-foreground">
            {slotPriceCoins} coin per slot
          </p>
        </div>
      </button>
    </li>
  );
}
