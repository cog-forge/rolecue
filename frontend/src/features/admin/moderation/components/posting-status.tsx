"use client";

import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import type { PostingStatus as Status } from "../schemas/moderation-schema";

const dotClass: Record<Status, string> = {
  pending: "bg-workspace-accent",
  open: "bg-foreground",
  intake_closed: "bg-muted-foreground",
  closed: "bg-muted-foreground",
  rejected: "bg-destructive",
};

// Status is always text plus a dot, never colour alone (team design rule). The value stays the
// API's; only the label is translated.
export function PostingStatus({
  status,
  className,
}: {
  status: Status;
  className?: string;
}) {
  const t = useTranslations("admin.moderation.status");
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 text-sm whitespace-nowrap",
        className,
      )}
    >
      <span
        aria-hidden
        className={cn("size-2 shrink-0 rounded-full", dotClass[status])}
      />
      {t(status)}
    </span>
  );
}
