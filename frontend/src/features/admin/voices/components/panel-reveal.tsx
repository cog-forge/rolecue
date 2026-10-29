"use client";

import { useEffect, useState, type ReactNode } from "react";
import { cn } from "@/lib/utils";

// Plays the panel reveal once on mount. Give it a key so it plays again when the content changes.
export function PanelReveal({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  const [open, setOpen] = useState(false);
  useEffect(() => {
    const frame = requestAnimationFrame(() => setOpen(true));
    return () => cancelAnimationFrame(frame);
  }, []);
  return (
    <div className={cn("t-panel-slide", className)} data-open={open}>
      {children}
    </div>
  );
}
