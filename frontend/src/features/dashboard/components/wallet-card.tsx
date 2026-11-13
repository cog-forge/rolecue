"use client";

import { useTranslations } from "next-intl";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { getWallet, walletKeys } from "@/features/billing/api/wallet";
import { CardError, CardLoading, DashboardCard } from "./dashboard-card";
import { useFormatters } from "@/lib/i18n/use-formatters";

// The balance is the backend's authoritative figure; the card only displays it.
export function WalletCard({ className }: { className?: string }) {
  const t = useTranslations("candidate.dashboard.wallet");
  const fmt = useFormatters();
  const user = useSessionUser();
  const query = useQuery({
    queryKey: walletKeys.wallet(user.id),
    queryFn: ({ signal }) => getWallet(signal),
    retry: false,
  });

  return (
    <DashboardCard
      title={t("title")}
      className={className}
      isLoading={query.isPending}
      compact
    >
      {query.isPending ? (
        <CardLoading label={t("loading")} />
      ) : query.isError ? (
        <CardError
          message={t("error")}
          disabled={query.isFetching}
          onRetry={() => void query.refetch()}
        />
      ) : (
        <div className="space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <p className="text-3xl font-semibold tracking-tight tabular-nums">
              {fmt.coinAmount(query.data.balance)}{" "}
              <span className="text-sm font-medium text-muted-foreground">
                {t("unit")}
              </span>
            </p>
            <Button
              asChild
              variant="outline"
              className="h-11 bg-background text-xs"
            >
              <Link href={routes.billing}>{t("view")}</Link>
            </Button>
          </div>
          <p className="text-xs leading-relaxed text-muted-foreground">
            {t("description")}
          </p>
        </div>
      )}
    </DashboardCard>
  );
}
