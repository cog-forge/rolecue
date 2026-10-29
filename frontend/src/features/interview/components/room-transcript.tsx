"use client";
import { useMemo } from "react";
import { ChevronDown, ChevronUp, MessageSquare } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  MessageScroller,
  type MessageScrollerMessage,
} from "@/components/ui/message-scroller";
import {
  GlassMessage,
  GlassMessageContent,
} from "@/components/ui/glass-message";

/** Only real runtime captions belong here. No composer or generated interview turns. */
export type TranscriptEntry = {
  id: string;
  speaker: "interviewer" | "candidate";
  text: string;
  partial?: boolean;
};
export function RoomTranscript({
  expanded,
  onExpand,
  entries = [],
}: {
  expanded: boolean;
  onExpand: () => void;
  entries?: readonly TranscriptEntry[];
}) {
  const messages = useMemo(
    () =>
      entries.map((entry) => ({
        id: entry.id,
        from:
          entry.speaker === "interviewer"
            ? ("assistant" as const)
            : ("user" as const),
        content: entry.text,
      })),
    [entries],
  );
  const renderMessage = (message: MessageScrollerMessage) => {
    const entry = entries.find((entry) => entry.id === message.id);
    return (
      <GlassMessage
        from={message.from === "user" ? "user" : "assistant"}
        name={message.from === "user" ? "You" : "Interviewer"}
        footer={entry?.partial ? "Transcribing…" : undefined}
        className="text-white"
      >
        <GlassMessageContent
          blur={12}
          refraction={4}
          dispersion={0}
          specular={0.1}
          tint={
            message.from === "user"
              ? "rgba(24,92,192,.35)"
              : "rgba(255,255,255,.1)"
          }
          className="max-w-full text-sm leading-relaxed [--glass-enter-scale:0.98] [--glass-enter-y:4px]"
        >
          {message.content}
        </GlassMessageContent>
      </GlassMessage>
    );
  };
  return (
    <>
      <header className="flex shrink-0 items-center gap-3 border-b border-white/10 px-5 py-4">
        <span className="flex size-9 items-center justify-center rounded-full border border-white/10 bg-white/5">
          <MessageSquare
            aria-hidden
            className="size-4 text-white/75"
            strokeWidth={1.5}
          />
        </span>
        <div className="min-w-0 flex-1">
          <h2 className="text-sm font-medium">Live transcript</h2>
          <p className="mt-1 text-[11px] text-white/60">
            Questions & your spoken answers
          </p>
        </div>
        <Button
          variant="ghost"
          size="icon"
          aria-label={expanded ? "Reduce transcript" : "Expand transcript"}
          aria-expanded={expanded}
          onClick={onExpand}
          className="size-11 rounded-full text-white/75 hover:bg-white/10 hover:text-white md:hidden"
        >
          {expanded ? <ChevronDown aria-hidden /> : <ChevronUp aria-hidden />}
        </Button>
      </header>
      <MessageScroller
        aria-label="Live transcript"
        className="h-auto min-h-0 flex-1 [--foreground:#fafafa] [--muted-foreground:#d4d4d8]"
        messages={messages}
        renderMessage={renderMessage}
        emptyState={
          <div className="space-y-5 py-4">
            <p className="text-[11px] text-white/55">Room preview</p>
            <GlassMessage name="Interview room" className="text-white">
              <GlassMessageContent
                blur={12}
                refraction={4}
                dispersion={0}
                tint="rgba(255,255,255,.08)"
                className="max-w-full text-sm leading-relaxed [--glass-enter-scale:0.98] [--glass-enter-y:4px]"
              >
                Questions and your spoken answers will appear here when your
                live interview is connected.
              </GlassMessageContent>
            </GlassMessage>
            <p className="px-1 text-xs leading-relaxed text-white/60">
              Live captions are unavailable in this preview.
            </p>
          </div>
        }
      />
      <footer className="shrink-0 border-t border-white/10 px-5 py-3 text-[11px] leading-relaxed text-white/65">
        Transcript only · Your microphone stays local in this preview.
      </footer>
    </>
  );
}
