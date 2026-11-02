"use client";

import { Check, FileText, LoaderCircle, PencilLine } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { useTranslations } from "next-intl";
import type { RoleSummary } from "../api/contract";
import { statusLabel } from "../lib/labels";

const statusIcons = {
  draft: FileText,
  analyzing: LoaderCircle,
  review: PencilLine,
  confirmed: Check,
} as const;

export function RoleProfileStatus({
  status,
}: {
  status: RoleSummary["status"];
}) {
  const t = useTranslations("candidate.roleProfiles.labels");
  const Icon = statusIcons[status];
  return (
    <Badge
      variant={status === "confirmed" ? "positive" : "secondary"}
      className="h-auto min-h-6 gap-1.5 py-0.5 whitespace-normal"
    >
      <Icon
        className={status === "analyzing" ? "motion-safe:animate-spin" : ""}
        aria-hidden="true"
      />
      {statusLabel(t, status)}
    </Badge>
  );
}
