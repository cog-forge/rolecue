"use client";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { OnboardingDialog } from "./onboarding-dialog";
export function OnboardingGate({ children }: { children: React.ReactNode }) {
  const user = useSessionUser();
  if (user.role !== "admin" && !user.onboarding_completed)
    return <OnboardingDialog user={user} />;
  return children;
}
