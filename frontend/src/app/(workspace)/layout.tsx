import { OnboardingGate } from "@/features/onboarding/components/onboarding-gate";
import { SessionGate } from "@/features/auth/components/session-gate";
import { AuthenticatedShell } from "@/features/auth/components/authenticated-shell";

export default function WorkspaceLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <SessionGate>
      <OnboardingGate>
        <AuthenticatedShell>{children}</AuthenticatedShell>
      </OnboardingGate>
    </SessionGate>
  );
}
