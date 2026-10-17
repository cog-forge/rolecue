import { useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { UserAvatar } from "@/components/account/user-avatar";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { accountsKeys, getAccount, isAdminAccessError } from "../api/accounts";
import { AccountRoleBadge, AccountLockBadge } from "./account-badges";
import { accountDate, accountName } from "./accounts-table";
import { isAccountLocked } from "../schemas/account-state";
import { useAccountClock } from "../hooks/use-account-clock";

export function AccountDetailsSheet({
  id,
  onClose,
  onRestoreFocus,
  onAccessError,
}: {
  id: string;
  onClose: () => void;
  onRestoreFocus: () => void;
  onAccessError: () => Promise<void>;
}) {
  const viewer = useSessionUser();
  const detail = useQuery({
    queryKey: accountsKeys.detail(viewer.id, id),
    queryFn: ({ signal }) => getAccount(id, signal),
    staleTime: 0,
    retry: false,
    refetchOnWindowFocus: true,
  });
  const denied = isAdminAccessError(detail.error);
  useEffect(() => {
    if (denied) void onAccessError();
  }, [denied, onAccessError]);
  const user = !detail.error ? detail.data : undefined;
  const now = useAccountClock(user ? [user] : [], detail.dataUpdatedAt);
  const locked = user ? isAccountLocked(user, now) : false;
  const expired = user?.banned && !locked && !!user.banExpires;
  return (
    <Sheet
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <SheetContent
        className="w-full overflow-y-auto sm:max-w-lg"
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          onRestoreFocus();
        }}
      >
        <SheetHeader className="border-b p-6">
          <SheetTitle>Account details</SheetTitle>
          <SheetDescription>
            Current account information and access status.
          </SheetDescription>
        </SheetHeader>
        <div className="space-y-6 p-6">
          {detail.isPending && (
            <p role="status" className="text-sm text-muted-foreground">
              Loading account details…
            </p>
          )}
          {detail.error && (
            <div className="space-y-3" role="alert">
              <p className="text-sm">
                Unable to load account details. Please try again.
              </p>
              <Button
                variant="outline"
                className="h-11"
                disabled={detail.isFetching}
                onClick={() => void detail.refetch()}
              >
                Retry details
              </Button>
            </div>
          )}
          {user && (
            <>
              <div className="flex items-center gap-3">
                <UserAvatar
                  id={user.id}
                  name={accountName(user)}
                  image={user.image}
                />
                <div className="min-w-0">
                  <h2 className="break-words text-lg font-semibold">
                    {accountName(user)}
                  </h2>
                  <p className="break-all text-sm text-muted-foreground">
                    {user.email}
                  </p>
                </div>
              </div>
              <div className="flex flex-wrap gap-2">
                <AccountRoleBadge role={user.role} />
                <AccountLockBadge user={user} checkedAt={now} />
              </div>
              <dl className="divide-y text-sm">
                {[
                  ["Account ID", user.id],
                  [
                    "Email verification",
                    user.emailVerified === undefined ||
                    user.emailVerified === null
                      ? "Not available"
                      : user.emailVerified
                        ? "Verified"
                        : "Not verified",
                  ],
                  ["Created", accountDate(user.createdAt)],
                  [
                    "Last updated",
                    user.updatedAt
                      ? accountDate(user.updatedAt)
                      : "Not available",
                  ],
                  [
                    "Lock duration",
                    locked
                      ? user.banExpires
                        ? `Until ${accountDate(user.banExpires)}`
                        : "Indefinitely"
                      : "No current lock",
                  ],
                ].map(([label, value]) => (
                  <div key={label} className="space-y-1 py-4">
                    <dt className="text-xs text-muted-foreground">{label}</dt>
                    <dd className="break-words">{value}</dd>
                  </div>
                ))}
              </dl>
              {expired && user.banExpires && (
                <p className="text-sm text-muted-foreground">
                  Previous lock expired on {accountDate(user.banExpires)}. This
                  account is active; previously revoked sessions remain invalid.
                </p>
              )}
              {user.banned && (
                <div className="space-y-2 rounded-lg border bg-muted/30 p-4">
                  <h3 className="text-sm font-medium">
                    {expired ? "Previous lock reason" : "Recorded lock reason"}
                  </h3>
                  <p className="break-words text-sm">
                    {user.banReason || "No reason recorded"}
                  </p>
                </div>
              )}
              {user.role === "admin" && (
                <p className="text-xs text-muted-foreground">
                  Administrator accounts are read only and cannot be locked or
                  unlocked here.
                </p>
              )}
            </>
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}
