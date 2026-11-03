"use client";
import Image from "next/image";
import { useTranslations } from "next-intl";
import { ImageIcon, Check } from "lucide-react";
import { Toolbar, RadioGroup as RadioGroupPrimitive } from "radix-ui";
import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { RadioGroup } from "@/components/ui/radio-group";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { EnvironmentStatus } from "@/components/avatar-viewer/environment/environment-state";
import { useFormatters } from "@/lib/i18n/use-formatters";
import {
  roomBackgrounds,
  type RoomBackground,
  type RoomBackgroundId,
} from "./room-backgrounds";
import { roomGlass, roomIconButton } from "./room-control-button";

export function RoomBackgroundPicker({
  value,
  onChange,
  sceneStatus,
}: {
  value: RoomBackgroundId;
  onChange: (value: RoomBackgroundId) => void;
  /** Status of the selected 3D room; `off` for a photo. */
  sceneStatus: EnvironmentStatus;
}) {
  const t = useTranslations("candidate.interviewRoom.background");
  const fmt = useFormatters();
  const loading = sceneStatus.state === "loading" ? sceneStatus.progress : null;
  const tile = (background: RoomBackground) => (
    <RadioGroupPrimitive.Item
      key={background.id}
      value={background.id}
      aria-label={
        background.scene
          ? t("sceneLabel", { name: t(`options.${background.id}`) })
          : undefined
      }
      className="group relative cursor-pointer overflow-hidden rounded-xl border border-white/15 p-1 transition-colors data-[state=checked]:border-white/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/70 motion-reduce:transition-none"
    >
      <Image
        src={background.thumbnail}
        alt=""
        width={320}
        height={180}
        className="aspect-video w-full rounded-lg object-cover"
      />
      <span className="block py-2 text-center text-xs text-white/90">
        {t(`options.${background.id}`)}
      </span>
      {background.scene && (
        <span
          aria-hidden
          className="absolute top-2 left-2 rounded-full bg-black/55 px-1.5 py-0.5 text-[9px] font-medium tracking-wide text-white"
        >
          {t("badge")}
        </span>
      )}
      {value === background.id && (
        <span className="absolute top-2 right-2 flex size-4 items-center justify-center rounded-full bg-white text-neutral-950">
          <Check aria-hidden className="size-3" />
        </span>
      )}
    </RadioGroupPrimitive.Item>
  );
  return (
    <Popover>
      <Tooltip>
        <TooltipTrigger asChild>
          <PopoverTrigger asChild>
            <Toolbar.Button asChild>
              <Button
                variant="ghost"
                size="icon"
                className={`relative ${roomIconButton}`}
                aria-label={t("choose")}
              >
                <ImageIcon
                  aria-hidden
                  className="size-[45%]"
                  strokeWidth={1.5}
                />
                {loading !== null && (
                  <svg
                    aria-hidden
                    data-testid="scene-progress-ring"
                    viewBox="0 0 36 36"
                    className="pointer-events-none absolute inset-0 size-full -rotate-90"
                  >
                    <circle
                      cx="18"
                      cy="18"
                      r="16"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="2"
                      pathLength={100}
                      strokeDasharray={`${Math.max(4, loading * 100)} 100`}
                      className="text-white/80 transition-[stroke-dasharray] motion-reduce:transition-none"
                    />
                  </svg>
                )}
              </Button>
            </Toolbar.Button>
          </PopoverTrigger>
        </TooltipTrigger>
        <TooltipContent
          className="motion-reduce:animate-none!"
          side="top"
          sideOffset={14}
        >
          {t("choose")}
        </TooltipContent>
      </Tooltip>
      <PopoverContent
        aria-label={t("panel")}
        side="top"
        sideOffset={20}
        collisionPadding={12}
        className={`w-80 rounded-3xl p-4 motion-reduce:animate-none! ${roomGlass}`}
      >
        <h2 className="text-sm font-medium">{t("title")}</h2>
        <p className="mt-1 mb-4 text-xs text-white/60">{t("intro")}</p>
        <RadioGroup
          aria-label={t("group")}
          value={value}
          onValueChange={(id) => {
            const option = roomBackgrounds.find((item) => item.id === id);
            if (option) onChange(option.id);
          }}
          className="gap-3"
        >
          <h3 className="text-[11px] font-medium tracking-wide text-white/60 uppercase">
            {t("scenes")}
          </h3>
          <div className="grid grid-cols-3 gap-2">
            {roomBackgrounds.filter((item) => item.scene).map(tile)}
          </div>
          <h3 className="text-[11px] font-medium tracking-wide text-white/60 uppercase">
            {t("photos")}
          </h3>
          <div className="grid grid-cols-3 gap-2">
            {roomBackgrounds.filter((item) => !item.scene).map(tile)}
          </div>
        </RadioGroup>
        <p role="status" className="mt-3 min-h-4 text-xs text-white/75">
          {loading !== null
            ? t("loading", { percent: fmt.percent(loading * 100, 0) })
            : sceneStatus.state === "fallback"
              ? t(`fallback.${sceneStatus.reason}`)
              : null}
        </p>
        <p className="mt-2 text-[11px] text-white/50">
          {t.rich("credits", {
            link: (chunks) => (
              <a
                href="https://polyhaven.com"
                target="_blank"
                rel="noreferrer"
                className="underline underline-offset-2 hover:text-white"
              >
                {chunks}
              </a>
            ),
          })}
        </p>
      </PopoverContent>
    </Popover>
  );
}
