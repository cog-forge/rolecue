"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { authClient } from "@/lib/auth/client";
import { Button } from "@/components/ui/button";
import { gooeyToast } from "goey-toast";

export function AccountLockAction({
  userId,
  locked,
}: {
  userId: string;
  locked: boolean;
}) {
  const cache = useQueryClient();
  const action = useMutation({
    mutationFn: async () => {
      const result = locked
        ? await authClient.admin.unbanUser({ userId })
        : await authClient.admin.banUser({
            userId,
            banReason: "Locked by an administrator",
          });
      if (result.error)
        throw new Error(result.error.message || "Unable to update account");
    },
    onSuccess: async () => {
      gooeyToast.success(locked ? "Account unlocked" : "Account locked", {
        description: locked
          ? "This user can sign in again."
          : "This user's active sessions have been revoked.",
      });
      await cache.invalidateQueries({ queryKey: ["auth", "admin-users"] });
    },
    onError: (error) => {
      gooeyToast.error("Account could not be updated", {
        description: error.message,
      });
    },
  });
  return (
    <div className="space-y-2">
      <Button
        variant="outline"
        disabled={action.isPending}
        onClick={() => action.mutate()}
      >
        {action.isPending
          ? "Updating…"
          : locked
            ? "Unlock account"
            : "Lock account"}
      </Button>
      {action.error && (
        <p role="alert" className="text-sm text-destructive">
          {action.error.message}
        </p>
      )}
    </div>
  );
}
