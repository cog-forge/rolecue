"use client";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { OnboardingDialog } from "./onboarding-dialog";
export function OnboardingGate({ children }: { children: React.ReactNode }) {
  const user = useSessionUser();
  if (user.role !== "admin" && !user.onboarding_completed)
    return (
      <>
        <div
          aria-hidden="true"
          inert
          className="pointer-events-none select-none"
        >
          {children}
        </div>
        <OnboardingDialog user={user} />
      </>
    );
  return children;
}
