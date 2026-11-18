import { RoleGate } from "@/features/auth/components/role-gate";
export default function Layout({ children }: { children: React.ReactNode }) {
  return <RoleGate role={["candidate", "recruiter"]}>{children}</RoleGate>;
}
