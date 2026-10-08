import { RoleGate } from "@/features/auth/components/role-gate";

export default function CandidateLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <RoleGate role="candidate">{children}</RoleGate>;
}
