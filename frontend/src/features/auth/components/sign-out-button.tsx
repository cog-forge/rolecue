"use client";

import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { useSignOut } from "../hooks/use-sign-out";

export function SignOutButton() {
  const t = useTranslations("auth");
  const { signOut, pending, error } = useSignOut();
  return (
    <div className="space-y-2">
      <Button
        variant="outline"
        disabled={pending}
        onClick={() => void signOut()}
      >
        {pending ? t("onboarding.signingOut") : t("onboarding.signOut")}
      </Button>
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {t("verification.signOutFailed")}
        </p>
      )}
    </div>
  );
}
