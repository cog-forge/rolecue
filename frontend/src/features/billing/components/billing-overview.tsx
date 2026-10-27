"use client";

import { Suspense, useCallback, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Plus, ShieldAlert, Wallet } from "lucide-react";
import { DialogTrigger } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { getTransactions, syncDepositStatus, walletKeys } from "../api/wallet";
import {
  PaymentStatusBanner,
  type PaymentReturnEvent,
} from "./payment-status-banner";
import { TransactionHistory } from "./transaction-history";
import { AddCoinsDialog } from "./wallet-purchase";

export function BillingOverview() {
  const user = useSessionUser();
  const queryClient = useQueryClient();

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
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div className="flex items-center gap-2">
            <span className="flex h-7 w-7 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <Wallet className="h-4 w-4" />
            </span>
            <h1 className="text-2xl font-bold tracking-tight text-foreground">
              Transaction history
            </h1>
          </div>
          <p className="mt-1 text-xs text-muted-foreground">
            Track your coin purchases, spending, and refunds.
          </p>
        </div>
        <AddCoinsDialog
          open={addingCoins}
          onOpenChange={setAddingCoins}
          userId={user.id}
          role={user.role}
        >
          <DialogTrigger asChild>
            <Button className="h-11 self-start sm:self-auto">
              <Plus aria-hidden /> Add coins
            </Button>
          </DialogTrigger>
        </AddCoinsDialog>
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
