"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import type { ProductRole } from "@/config/permissions";
import { workspaceHome } from "@/config/navigation";
import { useSessionUser } from "./session-gate";

// SessionGate in the workspace layout owns session loading and verification.
export function RoleGate({
  children,
  role,
}: {
  children: React.ReactNode;
  role: ProductRole;
}) {
  const user = useSessionUser();
  const router = useRouter();
  const pathname = usePathname();
  const allowed = user.role === role;

  useEffect(() => {
    if (!allowed) router.replace(workspaceHome(user.role));
  }, [allowed, user.role, router, pathname]);

  if (!allowed) return <p role="status">Opening your workspace…</p>;
  return children;
}
