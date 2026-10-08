"use client";

import { Button } from "@/components/ui/button";
import { useSignOut } from "../hooks/use-sign-out";

export function SignOutButton() {
  const { signOut, pending, error } = useSignOut();
  return (
    <div className="space-y-2">
      <Button
        variant="outline"
        disabled={pending}
        onClick={() => void signOut()}
      >
        {pending ? "Signing out…" : "Sign out"}
      </Button>
      {error && (
        <p role="alert" className="text-sm text-destructive">
          Sign out did not complete. Please try again.
        </p>
      )}
    </div>
  );
}
