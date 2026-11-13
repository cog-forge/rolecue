import { ErrorState } from "@/components/ui/error-state";
import { useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
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
import { useFormatters } from "@/lib/i18n/use-formatters";

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
  const fmt = useFormatters();
  const t = useTranslations("admin.accounts");
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
          <SheetTitle>{t("details.title")}</SheetTitle>
          <SheetDescription>{t("details.description")}</SheetDescription>
        </SheetHeader>
        <div className="space-y-6 p-6">
          {detail.isPending && (
            <p role="status" className="text-sm text-muted-foreground">
              {t("details.loading")}
            </p>
          )}
          {detail.error && (
            <ErrorState
              description={t("details.loadFailed")}
              onRetry={() => void detail.refetch()}
              retryLabel={t("details.retry")}
              isRetrying={detail.isFetching}
            ></ErrorState>
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
                  [t("details.accountId"), user.id],
                  [
                    t("details.emailVerification"),
                    user.emailVerified === undefined ||
                    user.emailVerified === null
                      ? t("details.notAvailable")
                      : user.emailVerified
                        ? t("details.verified")
                        : t("details.notVerified"),
                  ],
                  [t("details.created"), accountDate(user.createdAt, fmt)],
                  [
                    t("details.lastUpdated"),
                    user.updatedAt
                      ? accountDate(user.updatedAt, fmt)
                      : t("details.notAvailable"),
                  ],
                  [
                    t("details.lockDuration"),
                    locked
                      ? user.banExpires
                        ? t("lockState.until", {
                            date: accountDate(user.banExpires, fmt),
                          })
                        : t("lockState.indefinitely")
                      : t("details.noCurrentLock"),
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
                  {t("details.expired", {
                    date: accountDate(user.banExpires, fmt),
                  })}
                </p>
              )}
              {user.banned && (
                <div className="space-y-2 rounded-lg border bg-muted/30 p-4">
                  <h3 className="text-sm font-medium">
                    {expired
                      ? t("details.previousReason")
                      : t("details.recordedReason")}
                  </h3>
                  <p className="break-words text-sm">
                    {user.banReason || t("lockState.noReason")}
                  </p>
                </div>
              )}
              {user.role === "admin" && (
                <p className="text-xs text-muted-foreground">
                  {t("details.adminReadOnly")}
                </p>
              )}
            </>
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}
