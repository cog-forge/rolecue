"use client";

import { useState } from "react";
import {
  Coins,
  ExternalLink,
  Loader2,
  Sparkles,
  AlertCircle,
  RefreshCw,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import type { CoinPackage } from "../schemas/wallet-schema";

interface PackageGridProps {
  packages?: CoinPackage[];
  isLoading: boolean;
  isFetching: boolean;
  error: Error | null;
  onRefresh: () => void;
  onCheckout: (pkg: CoinPackage) => void;
  isCheckingOut: boolean;
  checkingOutPackageId?: string | null;
  checkoutError?: string | null;
}

export function PackageGrid({
  packages,
  isLoading,
  isFetching,
  error,
  onRefresh,
  onCheckout,
  isCheckingOut,
  checkingOutPackageId,
  checkoutError,
}: PackageGridProps) {
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const activePackages = packages?.filter((p) => p.is_active !== false) ?? [];

  const formatVND = (price: number) => {
    return new Intl.NumberFormat("vi-VN", {
      style: "currency",
      currency: "VND",
    }).format(price);
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h3 className="text-lg font-semibold tracking-tight text-foreground">
            Top-up Coin Packages
          </h3>
          <p className="text-xs text-muted-foreground">
            Select a package to purchase coins via PayOS (VietQR, Banking &
            Cards).
          </p>
        </div>
      </div>

      {checkoutError && (
        <div
          role="alert"
          className="flex items-center gap-2 rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-xs text-destructive"
        >
          <AlertCircle className="h-4 w-4 shrink-0" />
          <span>{checkoutError}</span>
        </div>
      )}

      {isLoading ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {[1, 2, 3].map((i) => (
            <Card key={i} className="border-border/70 p-6">
              <div className="space-y-4">
                <Skeleton className="h-5 w-24" />
                <Skeleton className="h-8 w-32" />
                <Skeleton className="h-4 w-44" />
                <Skeleton className="h-10 w-full pt-4" />
              </div>
            </Card>
          ))}
        </div>
      ) : error ? (
        <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border/80 p-8 text-center">
          <p className="text-sm font-medium text-destructive">
            Failed to load coin packages
          </p>
          <p className="mt-1 text-xs text-muted-foreground">
            {error.message || "Please check your network and try again."}
          </p>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={onRefresh}
            disabled={isFetching}
            className="mt-4 gap-2 text-xs"
          >
            <RefreshCw
              className={`h-3.5 w-3.5 ${isFetching ? "animate-spin" : ""}`}
            />
            Retry loading packages
          </Button>
        </div>
      ) : activePackages.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border/80 p-8 text-center text-muted-foreground">
          <Coins className="mx-auto h-8 w-8 opacity-40" />
          <p className="mt-2 text-sm font-medium">No packages available</p>
          <p className="text-xs text-muted-foreground">
            No active coin packages are currently listed. Please check back
            later.
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {activePackages.map((pkg, idx) => {
            const isSelected = selectedId === pkg.id;
            const isCurrentCheckingOut =
              isCheckingOut && checkingOutPackageId === pkg.id;
            const isPopular = idx === 1;

            return (
              <Card
                key={pkg.id}
                onClick={() => setSelectedId(pkg.id)}
                className={`relative flex flex-col justify-between overflow-hidden cursor-pointer rounded-xl border transition-all duration-200 hover:shadow-md ${
                  isSelected
                    ? "border-primary ring-2 ring-primary/20 shadow-sm bg-card"
                    : "border-border/80 hover:border-border bg-card/60"
                }`}
              >
                {isPopular && (
                  <div className="absolute right-3 top-3">
                    <Badge
                      variant="secondary"
                      className="gap-1 bg-primary/10 text-primary border-primary/20 text-[10px] font-semibold"
                    >
                      <Sparkles className="h-3 w-3" />
                      Popular
                    </Badge>
                  </div>
                )}

                <CardContent className="p-6">
                  <div className="space-y-4">
                    <div>
                      <h4 className="text-base font-bold text-foreground">
                        {pkg.name}
                      </h4>
                      {pkg.description && (
                        <p className="mt-1 text-xs text-muted-foreground line-clamp-2">
                          {pkg.description}
                        </p>
                      )}
                    </div>

                    <div className="rounded-lg bg-muted/50 p-3.5 text-center transition-colors">
                      <div className="flex items-center justify-center gap-1.5 text-2xl font-black tracking-tight text-foreground">
                        <Coins className="h-5 w-5 text-amber-500" />
                        <span>{pkg.coin_amount.toLocaleString("en-US")}</span>
                        <span className="text-xs font-bold text-muted-foreground">
                          COINS
                        </span>
                      </div>
                      <div className="mt-1 text-xs font-semibold text-muted-foreground">
                        {formatVND(pkg.price_vnd)}
                      </div>
                    </div>
                  </div>

                  <div className="mt-6 pt-2">
                    <Button
                      type="button"
                      variant={isSelected ? "default" : "outline"}
                      size="sm"
                      disabled={isCheckingOut}
                      onClick={(e) => {
                        e.stopPropagation();
                        setSelectedId(pkg.id);
                        onCheckout(pkg);
                      }}
                      className="w-full gap-2 text-xs font-semibold"
                    >
                      {isCurrentCheckingOut ? (
                        <>
                          <Loader2 className="h-3.5 w-3.5 animate-spin" />
                          <span>Preparing PayOS…</span>
                        </>
                      ) : (
                        <>
                          <span>Buy with PayOS</span>
                          <ExternalLink className="h-3.5 w-3.5 opacity-70" />
                        </>
                      )}
                    </Button>
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}
