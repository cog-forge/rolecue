import { Check, FileText, LoaderCircle, PencilLine } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { statusLabels, type RoleSummary } from "../api/contract";

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
  const Icon = statusIcons[status];
  return (
    <Badge
      variant={status === "confirmed" ? "positive" : "secondary"}
      className="h-6 gap-1.5"
    >
      <Icon
        className={status === "analyzing" ? "motion-safe:animate-spin" : ""}
        aria-hidden="true"
      />
      {statusLabels[status]}
    </Badge>
  );
}
