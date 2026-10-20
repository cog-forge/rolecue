"use client";

import { Suspense, useCallback, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { ShieldAlert, Wallet } from "lucide-react";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  getWallet,
  getCoinPackages,
  getTransactions,
  createDepositCheckout,
  syncDepositStatus,
  walletKeys,
} from "../api/wallet";
import type { CoinPackage } from "../schemas/wallet-schema";
import {
  PaymentStatusBanner,
  type PaymentReturnEvent,
} from "./payment-status-banner";
import { BalanceCard } from "./balance-card";
import { PackageGrid } from "./package-grid";
import { TransactionHistory } from "./transaction-history";

export function BillingOverview() {
  const user = useSessionUser();
  const queryClient = useQueryClient();

  const [page, setPage] = useState(1);
  const pageSize = 20;
  const [checkingOutPackageId, setCheckingOutPackageId] = useState<
    string | null
  >(null);
  const [checkoutError, setCheckoutError] = useState<string | null>(null);

  // 1. Authoritative Wallet Balance Query
  const walletQuery = useQuery({
    queryKey: walletKeys.wallet(user.id),
    queryFn: ({ signal }) => getWallet(signal),
    enabled: user.role !== "admin",
    staleTime: 30_000,
  });

  // 2. Authoritative Coin Packages Query
  const packagesQuery = useQuery({
    queryKey: walletKeys.packages(),
    queryFn: ({ signal }) => getCoinPackages(signal),
    enabled: user.role !== "admin",
    staleTime: 60_000,
  });

  // 3. Authoritative Transactions Query
  const transactionsQuery = useQuery({
    queryKey: walletKeys.transactions(user.id, page, pageSize),
    queryFn: ({ signal }) => getTransactions(page, pageSize, signal),
    enabled: user.role !== "admin",
    staleTime: 15_000,
  });

  // PayOS Checkout Mutation
  const checkoutMutation = useMutation({
    mutationFn: async (pkg: CoinPackage) => {
      setCheckingOutPackageId(pkg.id);
      setCheckoutError(null);

      const origin =
        typeof window !== "undefined" ? window.location.origin : "";
      const pathname =
        typeof window !== "undefined" ? window.location.pathname : "/billing";

      const returnUrl = `${origin}${pathname}?status=success`;
      const cancelUrl = `${origin}${pathname}?cancel=true`;

      return createDepositCheckout({
        package_id: pkg.id,
        return_url: returnUrl,
        cancel_url: cancelUrl,
      });
    },
    onSuccess: (data) => {
      if (typeof window !== "undefined" && data.checkout_url) {
        window.location.href = data.checkout_url;
      }
    },
    onError: (err: unknown) => {
      setCheckingOutPackageId(null);
      const message =
        err instanceof Error
          ? err.message
          : "Failed to initialize PayOS checkout session. Please try again.";
      setCheckoutError(message);
    },
  });

  const handleCheckout = (pkg: CoinPackage) => {
    checkoutMutation.mutate(pkg);
  };

  const handlePaymentSuccess = useCallback(() => {
    // Invalidate queries so fresh balance & transactions are immediately loaded
    void queryClient.invalidateQueries({ queryKey: walletKeys.all(user.id) });
    void queryClient.invalidateQueries({
      queryKey: walletKeys.wallet(user.id),
    });
    void queryClient.invalidateQueries({
      queryKey: walletKeys.transactions(user.id, page, pageSize),
    });
  }, [queryClient, user.id, page, pageSize]);

  // PayOS Return State Sync Mutation
  const syncMutation = useMutation({
    mutationFn: (code: number) => syncDepositStatus(code),
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: walletKeys.all(user.id) });
      void queryClient.invalidateQueries({
        queryKey: walletKeys.wallet(user.id),
      });
      void queryClient.invalidateQueries({
        queryKey: walletKeys.transactions(user.id, page, pageSize),
      });
    },
  });

  const handlePaymentReturn = useCallback(
    ({ orderCode }: PaymentReturnEvent) => {
      if (orderCode) {
        syncMutation.mutate(orderCode);
      } else {
        void queryClient.invalidateQueries({
          queryKey: walletKeys.all(user.id),
        });
        void queryClient.invalidateQueries({
          queryKey: walletKeys.wallet(user.id),
        });
        void queryClient.invalidateQueries({
          queryKey: walletKeys.transactions(user.id, page, pageSize),
        });
      }
    },
    [syncMutation, queryClient, user.id, page, pageSize],
  );

  // Admin has no personal wallet per contract
  if (user.role === "admin") {
    return (
      <main className="mx-auto max-w-5xl space-y-6 py-6 px-4 sm:px-6">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground">
            Wallet & Coins
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Coin balances and top-ups are reserved for candidate practice and
            recruiter posting workspaces.
          </p>
        </div>

        <div
          role="alert"
          className="flex items-start gap-3 rounded-xl border border-border/80 bg-muted/40 p-5 text-sm text-muted-foreground"
        >
          <ShieldAlert className="h-5 w-5 text-muted-foreground shrink-0 mt-0.5" />
          <div className="space-y-1">
            <h4 className="font-semibold text-foreground">
              Administrator Accounts Do Not Have a Personal Wallet
            </h4>
            <p>
              Platform payment audits and revenue monitoring are governed under
              the Administrator Finance portal.
            </p>
          </div>
        </div>
      </main>
    );
  }

  return (
    <main className="mx-auto max-w-5xl space-y-8 py-6 px-4 sm:px-6">
      {/* Page Header */}
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div className="flex items-center gap-2">
            <span className="flex h-7 w-7 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <Wallet className="h-4 w-4" />
            </span>
            <h1 className="text-2xl font-bold tracking-tight text-foreground">
              Coin Wallet & Top-up
            </h1>
          </div>
          <p className="mt-1 text-xs text-muted-foreground">
            Check your live balance, purchase coins securely through PayOS, and
            inspect your full ledger history.
          </p>
        </div>
      </div>

      {/* PayOS Return State Banner */}
      <Suspense fallback={null}>
        <PaymentStatusBanner
          onPaymentSuccess={handlePaymentSuccess}
          onPaymentReturn={handlePaymentReturn}
        />
      </Suspense>

      {/* Section 1: Current Balance Card */}
      <section aria-labelledby="wallet-balance-heading">
        <h2 id="wallet-balance-heading" className="sr-only">
          Current Balance
        </h2>
        <BalanceCard
          wallet={walletQuery.data}
          isLoading={walletQuery.isLoading}
          isFetching={walletQuery.isFetching}
          error={walletQuery.error}
          onRefresh={() => void walletQuery.refetch()}
        />
      </section>

      {/* Section 2: Available Coin Packages */}
      <section aria-labelledby="coin-packages-heading">
        <h2 id="coin-packages-heading" className="sr-only">
          Coin Packages
        </h2>
        <PackageGrid
          packages={packagesQuery.data}
          isLoading={packagesQuery.isLoading}
          isFetching={packagesQuery.isFetching}
          error={packagesQuery.error}
          onRefresh={() => void packagesQuery.refetch()}
          onCheckout={handleCheckout}
          isCheckingOut={checkoutMutation.isPending}
          checkingOutPackageId={checkingOutPackageId}
          checkoutError={checkoutError}
        />
      </section>

      {/* Section 3: Unified Transaction History */}
      <section aria-labelledby="transaction-history-heading">
        <h2 id="transaction-history-heading" className="sr-only">
          Transaction History
        </h2>
        <TransactionHistory
          currentUserId={user.id}
          data={transactionsQuery.data}
          isLoading={transactionsQuery.isLoading}
          isFetching={transactionsQuery.isFetching}
          error={transactionsQuery.error}
          page={page}
          onPageChange={setPage}
          onRefresh={() => void transactionsQuery.refetch()}
        />
      </section>
    </main>
  );
}
