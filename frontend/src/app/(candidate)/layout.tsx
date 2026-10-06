import { SessionGate } from "@/features/auth/components/session-gate";
import { SignOutButton } from "@/features/auth/components/sign-out-button";
import { SiteShell } from "@/components/layout/site-shell";
import { candidateNavigation } from "@/config/navigation";
export default function Layout({ children }: { children: React.ReactNode }) {
  return (
    <SessionGate>
      <SiteShell navigation={candidateNavigation} label="Candidate">
        <SignOutButton />
        {children}
      </SiteShell>
    </SessionGate>
  );
}
