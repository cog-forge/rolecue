import { RoleGate } from "@/features/auth/components/role-gate";

export default function AdminLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <RoleGate role="admin">{children}</RoleGate>;
}
