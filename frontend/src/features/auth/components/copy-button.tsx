"use client";

import { useEffect, useState, type ReactNode } from "react";
import { Check, Copy } from "lucide-react";
import { Button } from "@/components/ui/button";

// Copies a secret to the clipboard; the icon swaps to a check while it is confirmed.
export function CopyButton({
  value,
  label,
  children,
}: {
  value: string;
  label: string;
  children?: ReactNode;
}) {
  const [state, setState] = useState<"idle" | "copied" | "failed">("idle");
  useEffect(() => {
    if (state === "idle") return;
    const timer = window.setTimeout(() => setState("idle"), 2000);
    return () => window.clearTimeout(timer);
  }, [state]);
  return (
    <>
      <Button
        type="button"
        variant="outline"
        size={children ? "default" : "icon"}
        className={children ? "min-h-11" : "size-11 shrink-0"}
        aria-label={children ? undefined : label}
        onClick={async () => {
          try {
            await navigator.clipboard.writeText(value);
            setState("copied");
          } catch {
            setState("failed");
          }
        }}
      >
        <span
          className="t-icon-swap"
          data-state={state === "copied" ? "b" : "a"}
          aria-hidden
        >
          <Copy className="t-icon" data-icon="a" />
          <Check className="t-icon" data-icon="b" />
        </span>
        {children}
      </Button>
      <span role="status" className="sr-only">
        {state === "copied"
          ? "Copied to clipboard."
          : state === "failed"
            ? "Unable to copy. Select the text and copy it manually."
            : ""}
      </span>
    </>
  );
}
