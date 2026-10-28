"use client";

import { Suspense, useCallback, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { ShieldAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DialogTrigger } from "@/components/ui/dialog";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  getTransactions,
  getWallet,
  syncDepositStatus,
  walletKeys,
} from "../api/wallet";
import {
  PaymentStatusBanner,
  type PaymentReturnEvent,
} from "./payment-status-banner";
import { TransactionHistory } from "./transaction-history";
import { AddCoinsDialog } from "./wallet-purchase";

export function BillingOverview() {
  const user = useSessionUser();
  const queryClient = useQueryClient();

  const wallet = useQuery({
    queryKey: walletKeys.wallet(user.id),
    queryFn: ({ signal }) => getWallet(signal),
    enabled: user.role !== "admin",
    staleTime: 30_000,
  });

  const [page, setPage] = useState(1);
  const [addingCoins, setAddingCoins] = useState(false);
  const pageSize = 20;
  // 3. Authoritative Transactions Query
  const transactionsQuery = useQuery({
    queryKey: walletKeys.transactions(user.id, page, pageSize),
    queryFn: ({ signal }) => getTransactions(page, pageSize, signal),
    enabled: user.role !== "admin",
    staleTime: 15_000,
  });

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

  const { mutate: syncPayment } = syncMutation;
  const handlePaymentReturn = useCallback(
    ({ orderCode }: PaymentReturnEvent) => {
      if (orderCode) syncPayment(orderCode);
    },
    [syncPayment],
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
    <div className="mx-auto max-w-6xl space-y-6">
      {/* Page Header */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">
            Transaction history
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Coins you bought, spent and got back.
          </p>
        </div>
        <div className="flex items-end gap-4">
          <div role="group" aria-label="Coin balance" className="sm:text-right">
            <p className="text-xs text-muted-foreground">Balance</p>
            <p className="text-2xl font-semibold tracking-tight text-foreground tabular-nums">
              {wallet.data
                ? `${wallet.data.balance.toLocaleString("en-US")} coins`
                : "—"}
            </p>
          </div>
          <AddCoinsDialog
            open={addingCoins}
            onOpenChange={setAddingCoins}
            userId={user.id}
            role={user.role}
          >
            <DialogTrigger asChild>
              <Button type="button">Add coins</Button>
            </DialogTrigger>
          </AddCoinsDialog>
        </div>
      </div>

      {/* PayOS Return State Banner */}
      <Suspense fallback={null}>
        <PaymentStatusBanner
          verification={syncMutation}
          onPaymentReturn={handlePaymentReturn}
        />
      </Suspense>

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
    </div>
  );
}
