"use client";

// Opaline Glass Message (MIT), Copyright (c) 2026 Deepraj Bera.
// https://github.com/deepraj21/opaline
// License: docs/LICENSE-Opaline.txt

import * as React from "react";

import { cn } from "@/lib/utils";
import { LiquidGlass } from "@/components/ui/liquid-glass";

type MessageFrom = "user" | "assistant";

const MessageContext = React.createContext<{ from: MessageFrom }>({
  from: "assistant",
});

/** A scrollable column of messages. */
function GlassMessageList({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="glass-message-list"
      role="log"
      aria-live="polite"
      className={cn("flex flex-col gap-3", className)}
      {...props}
    />
  );
}

/**
 * One chat message. `user` messages sit on the right in a tinted glass
 * bubble; `assistant` messages sit on the left on clear glass.
 */
function GlassMessage({
  from = "assistant",
  avatar,
  name,
  footer,
  className,
  children,
  ...props
}: React.ComponentProps<"div"> & {
  from?: MessageFrom;
  /** An image URL, initials, or any node. */
  avatar?: React.ReactNode;
  /** Sender name shown above the bubble. */
  name?: React.ReactNode;
  /** Meta under the bubble, e.g. a time or "Read". */
  footer?: React.ReactNode;
}) {
  const mine = from === "user";
  return (
    <MessageContext.Provider value={{ from }}>
      <div
        data-slot="glass-message"
        data-from={from}
        className={cn(
          "group/message flex max-w-full items-end gap-2",
          mine ? "flex-row-reverse self-end" : "self-start",
          className,
        )}
        {...props}
      >
        {avatar !== undefined ? (
          <GlassMessageAvatar>{avatar}</GlassMessageAvatar>
        ) : null}
        <div
          className={cn(
            "flex min-w-0 flex-col gap-1",
            mine ? "items-end" : "items-start",
          )}
        >
          {name ? (
            <span className="px-3 text-[12px] font-medium opacity-60">
              {name}
            </span>
          ) : null}
          {children}
          {footer ? (
            <span className="px-3 text-[11.5px] opacity-50">{footer}</span>
          ) : null}
        </div>
      </div>
    </MessageContext.Provider>
  );
}

function GlassMessageAvatar({
  className,
  children,
  ...props
}: React.ComponentProps<"span">) {
  const src =
    typeof children === "string" && /^(https?:|\/|data:)/.test(children)
      ? children
      : null;
  return (
    <span
      data-slot="glass-message-avatar"
      className={cn(
        "grid size-8 shrink-0 place-items-center overflow-hidden rounded-full bg-(--glass-highlight) text-[12px] font-semibold text-(--glass-foreground) ring-1 ring-white/30",
        className,
      )}
      {...props}
    >
      {src ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img src={src} alt="" className="size-full object-cover" />
      ) : (
        children
      )}
    </span>
  );
}

/** The bubble. Several in a row read as one group. */
function GlassMessageContent({
  className,
  tint,
  ...props
}: React.ComponentProps<typeof LiquidGlass>) {
  const { from } = React.useContext(MessageContext);
  const mine = from === "user";
  return (
    <LiquidGlass
      data-slot="glass-message-content"
      variant={mine ? "clear" : "frosted"}
      tint={tint ?? (mine ? "oklch(0.6 0.2 255 / 0.82)" : undefined)}
      bezel={12}
      className={cn(
        "max-w-[min(32rem,85vw)] rounded-[20px] px-3.5 py-2 text-[15px] leading-snug tracking-[-0.01em] break-words whitespace-pre-wrap",
        "motion-safe:animate-[opaline-glass-in_420ms_cubic-bezier(0.34,1.3,0.64,1)] [--glass-enter-scale:0.6] [--glass-enter-y:8px]",
        mine
          ? "origin-bottom-right rounded-br-md text-white"
          : "origin-bottom-left rounded-bl-md text-(--glass-foreground)",
        className,
      )}
      {...props}
    />
  );
}

/** Three bouncing dots while the other side is typing. */
function GlassMessageTyping({
  className,
  ...props
}: React.ComponentProps<typeof LiquidGlass>) {
  return (
    <LiquidGlass
      data-slot="glass-message-typing"
      role="status"
      aria-label="Typing"
      variant="frosted"
      bezel={10}
      className={cn(
        "flex h-9 w-fit items-center gap-1 self-start rounded-[20px] rounded-bl-md px-3.5 text-(--glass-foreground)",
        className,
      )}
      {...props}
    >
      {[0, 1, 2].map((i) => (
        <span
          key={i}
          className="size-1.5 rounded-full bg-current opacity-60 motion-safe:animate-[opaline-typing_1.2s_ease-in-out_infinite]"
          style={{ animationDelay: `${i * 0.16}s` }}
        />
      ))}
    </LiquidGlass>
  );
}

export {
  GlassMessage,
  GlassMessageAvatar,
  GlassMessageContent,
  GlassMessageList,
  GlassMessageTyping,
  type MessageFrom,
};
