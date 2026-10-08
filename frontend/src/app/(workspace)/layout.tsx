import { SessionGate } from "@/features/auth/components/session-gate";
import { AuthenticatedShell } from "@/features/auth/components/authenticated-shell";

export default function WorkspaceLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <SessionGate>
      <AuthenticatedShell>{children}</AuthenticatedShell>
    </SessionGate>
  );
}
