"use client";

import { Coins, RefreshCw, ShieldCheck, Wallet } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import type { Wallet as WalletType } from "../schemas/wallet-schema";

interface BalanceCardProps {
  wallet?: WalletType;
  isLoading: boolean;
  isFetching: boolean;
  error: Error | null;
  onRefresh: () => void;
}

export function BalanceCard({
  wallet,
  isLoading,
  isFetching,
  error,
  onRefresh,
}: BalanceCardProps) {
  return (
    <Card className="relative overflow-hidden border-border/80 bg-gradient-to-br from-card via-card to-muted/40 shadow-sm transition-all duration-300">
      <CardContent className="p-6 md:p-8">
        <div className="flex flex-col gap-6 md:flex-row md:items-center md:justify-between">
          <div className="space-y-3">
            <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              <span className="flex h-5 w-5 items-center justify-center rounded-full bg-primary/10 text-primary">
                <Wallet className="h-3 w-3" />
              </span>
              <span>Available Coin Balance</span>
            </div>

            {isLoading ? (
              <div className="space-y-2 py-1">
                <Skeleton className="h-10 w-36" />
                <Skeleton className="h-4 w-56" />
              </div>
            ) : error ? (
              <div className="space-y-2 py-1">
                <p className="text-sm font-medium text-destructive">
                  Unable to load coin balance
                </p>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={onRefresh}
                  disabled={isFetching}
                  className="h-8 gap-1.5 text-xs"
                >
                  <RefreshCw
                    className={`h-3.5 w-3.5 ${isFetching ? "animate-spin" : ""}`}
                  />
                  Retry
                </Button>
              </div>
            ) : (
              <div className="space-y-1">
                <div className="flex items-baseline gap-2.5">
                  <span className="text-4xl font-extrabold tracking-tight text-foreground md:text-5xl">
                    {wallet?.balance?.toLocaleString("en-US") ?? 0}
                  </span>
                  <span className="flex items-center gap-1 rounded-full bg-primary/10 px-2.5 py-0.5 text-xs font-bold text-primary">
                    <Coins className="h-3.5 w-3.5" />
                    COINS
                  </span>
                </div>
                <p className="text-xs text-muted-foreground">
                  Use coins for AI interview practices, voice generation, and
                  candidate evaluations.
                </p>
              </div>
            )}
          </div>

          <div className="flex flex-col items-start gap-3 border-t border-border/60 pt-4 md:items-end md:border-t-0 md:pt-0">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={onRefresh}
              disabled={isLoading || isFetching}
              aria-label="Refresh balance"
              className="gap-2 text-xs text-muted-foreground hover:text-foreground"
            >
              <RefreshCw
                className={`h-3.5 w-3.5 ${isFetching ? "animate-spin" : ""}`}
              />
              <span>{isFetching ? "Refreshing…" : "Refresh balance"}</span>
            </Button>

            <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground/80">
              <ShieldCheck className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
              <span>Authoritative ledger & PayOS secure checkout</span>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
