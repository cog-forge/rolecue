"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { getWallet, walletKeys } from "@/features/billing/api/wallet";
import { CardError, CardLoading, DashboardCard } from "./dashboard-card";
import { useFormatters } from "@/lib/i18n/use-formatters";

// The balance is the backend's authoritative figure; the card only displays it.
export function WalletCard() {
  const fmt = useFormatters();
  const user = useSessionUser();
  const query = useQuery({
    queryKey: walletKeys.wallet(user.id),
    queryFn: ({ signal }) => getWallet(signal),
    retry: false,
  });

  return (
    <DashboardCard title="Coin balance">
      {query.isPending ? (
        <CardLoading label="Loading your coin balance…" />
      ) : query.isError ? (
        <CardError
          message="We couldn't load your coin balance."
          disabled={query.isFetching}
          onRetry={() => void query.refetch()}
        />
      ) : (
        <div className="space-y-3">
          <p className="text-2xl font-semibold tabular-nums">
            {fmt.coinAmount(query.data.balance)}{" "}
            <span className="text-sm font-medium text-muted-foreground">
              coins
            </span>
          </p>
          <p className="text-sm text-muted-foreground">
            Coins are used for AI interview practice.
          </p>
          <Button asChild variant="outline" className="h-11 bg-background">
            <Link href={routes.billing}>View transactions</Link>
          </Button>
        </div>
      )}
    </DashboardCard>
  );
}
