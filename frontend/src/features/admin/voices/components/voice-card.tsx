import { UserAvatar } from "@/components/account/user-avatar";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import type { Voice } from "../schemas/voice-schema";
import { usage } from "./remove-voice-dialog";
import { VoiceStatus } from "./voice-status";

// One voice in the gallery. The whole card is the button that selects it; actions live in the details.
export function VoiceCard({
  voice,
  selected,
  onSelect,
}: {
  voice: Voice;
  selected: boolean;
  onSelect: (voice: Voice) => void;
}) {
  return (
    <button
      type="button"
      aria-pressed={selected}
      onClick={() => onSelect(voice)}
      className={cn(
        "flex h-full w-full flex-col gap-3 rounded-xl border bg-card p-4 text-left transition-[border-color,background-color,box-shadow] duration-[var(--duration-quick)] outline-none hover:border-foreground/30 focus-visible:ring-3 focus-visible:ring-ring/50",
        selected &&
          "border-workspace-accent bg-muted/40 ring-2 ring-workspace-accent/25 hover:border-workspace-accent",
        !voice.is_active && !selected && "bg-muted/30",
      )}
    >
      <span className="flex items-start gap-3">
        <UserAvatar id={voice.id} name={voice.name} className="size-10" />
        <span className="min-w-0 flex-1">
          <span className="block font-medium [overflow-wrap:break-word]">
            {voice.name}
          </span>
          <span className="block text-xs text-muted-foreground">
            {voice.gender ?? "Gender not set"}
          </span>
        </span>
      </span>
      <span className="flex flex-wrap items-center justify-between gap-x-3 gap-y-2">
        <span className="flex flex-wrap gap-1.5 text-xs">
          {voice.locale && (
            <span className="rounded-full bg-muted px-2.5 py-1 font-medium">
              {voice.locale}
            </span>
          )}
          {voice.in_use && (
            <span className="rounded-full border px-2.5 py-1 text-muted-foreground">
              {usage(voice)}
            </span>
          )}
        </span>
        <VoiceStatus active={voice.is_active} />
      </span>
    </button>
  );
}

export function VoiceCardSkeleton() {
  return (
    <div className="flex h-full flex-col gap-3 rounded-xl border bg-card p-4">
      <div className="flex items-start gap-3">
        <Skeleton className="size-10 rounded-xl" />
        <div className="flex-1 space-y-2">
          <Skeleton className="h-4 w-2/3" />
          <Skeleton className="h-3 w-1/3" />
        </div>
      </div>
      <div className="flex gap-1.5">
        <Skeleton className="h-6 w-14 rounded-full" />
        <Skeleton className="h-6 w-20 rounded-full" />
      </div>
    </div>
  );
}
