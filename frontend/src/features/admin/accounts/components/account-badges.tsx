import { LockKeyhole, ShieldCheck, UnlockKeyhole } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { roleLabels } from "@/config/navigation";
import type { AdminAccount } from "../api/accounts";
import { isAccountLocked } from "../schemas/account-state";

export function AccountRoleBadge({ role }: { role: AdminAccount["role"] }) {
  const valid =
    role === "candidate" || role === "recruiter" || role === "admin";
  const colors =
    role === "candidate"
      ? "border-sky-200 bg-sky-50 text-sky-800 dark:border-sky-800 dark:bg-sky-950 dark:text-sky-200"
      : role === "recruiter"
        ? "border-violet-200 bg-violet-50 text-violet-800 dark:border-violet-800 dark:bg-violet-950 dark:text-violet-200"
        : "border-slate-200 bg-slate-100 text-slate-700 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200";
  return (
    <Badge variant="outline" className={`h-6 gap-1.5 ${colors}`}>
      {role === "admin" && <ShieldCheck aria-hidden />}
      {valid ? roleLabels[role] : "Unknown"}
    </Badge>
  );
}

export function AccountLockBadge({
  user,
  checkedAt,
}: {
  user: AdminAccount;
  checkedAt: number;
}) {
  const locked = isAccountLocked(user, checkedAt);
  const Icon = locked ? LockKeyhole : UnlockKeyhole;
  const colors = locked
    ? "border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-800 dark:bg-rose-950 dark:text-rose-200"
    : "border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-200";
  return (
    <Badge variant="outline" className={`h-6 gap-1.5 ${colors}`}>
      <Icon aria-hidden />
      {locked ? "Locked" : "Active"}
    </Badge>
  );
}
