"use client";
import Image from "next/image";
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
import { roomBackgrounds, type RoomBackgroundId } from "./room-backgrounds";
import { roomGlass, roomIconButton } from "./room-control-button";

export function RoomBackgroundPicker({
  value,
  onChange,
}: {
  value: RoomBackgroundId;
  onChange: (value: RoomBackgroundId) => void;
}) {
  return (
    <Popover>
      <Tooltip>
        <TooltipTrigger asChild>
          <PopoverTrigger asChild>
            <Toolbar.Button asChild>
              <Button
                variant="ghost"
                size="icon"
                className={roomIconButton}
                aria-label="Choose background"
              >
                <ImageIcon
                  aria-hidden
                  className="size-[45%]"
                  strokeWidth={1.5}
                />
              </Button>
            </Toolbar.Button>
          </PopoverTrigger>
        </TooltipTrigger>
        <TooltipContent
          className="motion-reduce:animate-none!"
          side="top"
          sideOffset={14}
        >
          Choose background
        </TooltipContent>
      </Tooltip>
      <PopoverContent
        aria-label="Room backgrounds"
        side="top"
        sideOffset={20}
        collisionPadding={12}
        className={`w-80 rounded-3xl p-4 motion-reduce:animate-none! ${roomGlass}`}
      >
        <h2 className="text-sm font-medium">A change of scenery</h2>
        <p className="mt-1 mb-4 text-xs text-white/60">
          Choose a setting for this visit.
        </p>
        <RadioGroup
          aria-label="Room background"
          value={value}
          onValueChange={(id) => {
            const option = roomBackgrounds.find((item) => item.id === id);
            if (option) onChange(option.id);
          }}
          className="grid grid-cols-3 gap-2"
        >
          {roomBackgrounds.map((background) => (
            <RadioGroupPrimitive.Item
              key={background.id}
              value={background.id}
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
                {background.name}
              </span>
              {value === background.id && (
                <span className="absolute top-2 right-2 flex size-4 items-center justify-center rounded-full bg-white text-neutral-950">
                  <Check aria-hidden className="size-3" />
                </span>
              )}
            </RadioGroupPrimitive.Item>
          ))}
        </RadioGroup>
      </PopoverContent>
    </Popover>
  );
}
