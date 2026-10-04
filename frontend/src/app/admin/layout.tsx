import { SessionGate } from "@/features/auth/components/session-gate";
import { SignOutButton } from "@/features/auth/components/sign-out-button";
import { SiteShell } from "@/components/layout/site-shell";
import { adminNavigation } from "@/config/navigation";
export default function Layout({ children }: { children: React.ReactNode }) {
  return (
    <SessionGate requireAdmin>
      <SiteShell navigation={adminNavigation} label="Admin">
        <SignOutButton />
        {children}
      </SiteShell>
    </SessionGate>
  );
}
