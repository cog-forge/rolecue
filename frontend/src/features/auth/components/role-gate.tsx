"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import type { ProductRole } from "@/config/permissions";
import { workspaceHome } from "@/config/navigation";
import { useSessionUser } from "./session-gate";

// SessionGate in the workspace layout owns session loading and verification.
export function RoleGate({
  children,
  role,
}: {
  children: React.ReactNode;
  role: ProductRole | readonly ProductRole[];
}) {
  const t = useTranslations("common.shell.session");
  const user = useSessionUser();
  const router = useRouter();
  const pathname = usePathname();
  const allowed =
    typeof role === "string" ? user.role === role : role.includes(user.role);

  useEffect(() => {
    if (!allowed) router.replace(workspaceHome(user.role));
  }, [allowed, user.role, router, pathname]);

  if (!allowed) return <p role="status">{t("openingWorkspace")}</p>;
  return children;
}
