"use client";
import Image from "next/image";
import { useId } from "react";
import { useTranslations } from "next-intl";
import { Check } from "lucide-react";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import {
  roomBackgrounds,
  type RoomBackground,
} from "@/features/interview/components/room-backgrounds";
import { cn } from "@/lib/utils";
import { ENVIRONMENT_ANCHOR } from "../lib/setup-blockers";

// The room the interview takes place in. The ids are the room picker's (KAN-275); the server keeps
// a copy and a test fails when the two lists differ. A stored id this list no longer knows shows as
// nothing selected: the readiness answer reports it, the picker never hides it by picking another.
export function EnvironmentPicker({
  value,
  onChange,
  disabled,
}: {
  value: string | null;
  onChange: (id: RoomBackground["id"]) => void;
  disabled?: boolean;
}) {
  const t = useTranslations("candidate.setup.environment");
  const room = useTranslations("candidate.interviewRoom.background");
  const ids = useId();
  const known = roomBackgrounds.some((item) => item.id === value);
  return (
    <fieldset
      id={ENVIRONMENT_ANCHOR}
      tabIndex={-1}
      className="scroll-mt-24 space-y-3 outline-none"
    >
      <legend className="font-semibold">{t("title")}</legend>
      <p className="text-sm text-muted-foreground">{t("description")}</p>
      <RadioGroup
        aria-label={t("group")}
        value={known ? (value ?? "") : ""}
        onValueChange={(id) => {
          const option = roomBackgrounds.find((item) => item.id === id);
          if (option) onChange(option.id);
        }}
        disabled={disabled}
        className="grid grid-cols-2 gap-3 sm:grid-cols-3"
      >
        {roomBackgrounds.map((item) => (
          <label
            key={item.id}
            htmlFor={`${ids}-${item.id}`}
            className={cn(
              "relative flex min-w-0 cursor-pointer flex-col overflow-hidden rounded-xl border bg-card transition-[border-color,box-shadow] duration-(--duration-fast) hover:border-foreground/30 has-focus-visible:ring-3 has-focus-visible:ring-ring/50 has-data-[state=checked]:border-foreground has-data-[state=checked]:shadow-[inset_0_0_0_1px_var(--foreground)] motion-reduce:transition-none",
              disabled && "cursor-not-allowed opacity-60",
            )}
          >
            <Image
              src={item.thumbnail}
              alt=""
              width={320}
              height={180}
              className="aspect-video w-full object-cover"
            />
            <span className="flex items-center justify-between gap-2 p-3 text-sm font-medium">
              <span className="truncate">{room(`options.${item.id}`)}</span>
              {item.scene && (
                <span className="rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                  {room("badge")}
                </span>
              )}
            </span>
            <RadioGroupItem
              id={`${ids}-${item.id}`}
              value={item.id}
              aria-label={
                item.scene
                  ? room("sceneLabel", { name: room(`options.${item.id}`) })
                  : room(`options.${item.id}`)
              }
              className="absolute top-2.5 left-2.5 bg-background"
            />
            {value === item.id && (
              <Check
                aria-hidden="true"
                className="absolute top-2.5 right-2.5 size-4 rounded-full bg-background p-0.5"
              />
            )}
          </label>
        ))}
      </RadioGroup>
    </fieldset>
  );
}
