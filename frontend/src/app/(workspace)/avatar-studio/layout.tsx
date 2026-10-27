import { RoleGate } from "@/features/auth/components/role-gate";

// Shared by Candidates and Recruiters; Admins have no personal avatar inventory.
export default function AvatarStudioLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <RoleGate role={["candidate", "recruiter"]}>{children}</RoleGate>;
}
