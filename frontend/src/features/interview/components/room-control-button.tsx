"use client";

import { Toolbar } from "radix-ui";
import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

export const roomGlass =
  "border border-white/15 bg-(--rolecue-surface-dark)/85 text-(--rolecue-on-dark) shadow-[0_12px_40px_#00000030,inset_0_1px_0_#ffffff18] backdrop-blur-2xl";
export const roomIconButton =
  "size-full min-h-11 min-w-11 rounded-full text-white/85 hover:bg-white/12 hover:text-white focus-visible:ring-white/60 aria-pressed:bg-white/15 aria-disabled:cursor-default aria-disabled:text-white/45 aria-disabled:active:translate-y-0 motion-reduce:transition-none motion-reduce:transform-none";

export function RoomControlButton({
  icon: Icon,
  label,
  className,
  ...props
}: { icon: LucideIcon; label: string } & React.ComponentProps<typeof Button>) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Toolbar.Button asChild>
          <Button
            variant="ghost"
            size="icon"
            className={cn(roomIconButton, className)}
            aria-label={label}
            {...props}
          >
            <Icon aria-hidden className="size-[45%]" strokeWidth={1.6} />
          </Button>
        </Toolbar.Button>
      </TooltipTrigger>
      <TooltipContent
        className="motion-reduce:animate-none!"
        side="top"
        sideOffset={12}
      >
        {label}
      </TooltipContent>
    </Tooltip>
  );
}
