"use client";

import {
  Coins,
  ArrowUpRight,
  Loader2,
  AlertCircle,
  RefreshCw,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { CoinIcon } from "@/components/icons/coin-icon";
import { Skeleton } from "@/components/ui/skeleton";
import type { CoinPackage } from "../schemas/wallet-schema";
import { cn } from "@/lib/utils";
import { useFormatters } from "@/lib/i18n/use-formatters";

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
  const fmt = useFormatters();
  const activePackages =
    packages?.filter((pkg) => pkg.is_active !== false) ?? [];
  return (
    <div className="space-y-4">
      {checkoutError && (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-xl border border-destructive/30 bg-destructive/10 p-4 text-sm text-destructive"
        >
          <AlertCircle className="mt-0.5 size-4 shrink-0" aria-hidden />
          {checkoutError}
        </div>
      )}
      {isLoading ? (
        <div className="grid gap-3 sm:grid-cols-3">
          {[1, 2, 3].map((i) => (
            <Card key={i} className="min-h-[250px] p-5">
              <Skeleton className="h-6 w-28 motion-reduce:animate-none" />
              <Skeleton className="mt-8 h-16 w-36 motion-reduce:animate-none" />
              <Skeleton className="h-8 w-28 motion-reduce:animate-none" />
              <Skeleton className="mt-auto h-11 w-full motion-reduce:animate-none" />
            </Card>
          ))}
        </div>
      ) : error ? (
        <div
          role="alert"
          className="rounded-2xl border border-dashed p-8 text-center"
        >
          <p className="font-medium">Unable to load coin packages</p>
          <p className="mt-2 text-sm text-muted-foreground">
            Please try again in a moment.
          </p>
          <Button
            variant="outline"
            disabled={isFetching}
            onClick={onRefresh}
            className="mt-4 h-11"
          >
            <RefreshCw className="size-4" aria-hidden />
            Retry loading packages
          </Button>
        </div>
      ) : activePackages.length === 0 ? (
        <div className="rounded-2xl border border-dashed p-10 text-center">
          <Coins className="mx-auto size-8 text-muted-foreground" aria-hidden />
          <p className="mt-3 font-medium">No packages available</p>
          <p className="mt-1 text-sm text-muted-foreground">
            Please check back later.
          </p>
        </div>
      ) : (
        <div className="grid gap-3 sm:grid-cols-3">
          {activePackages.map((pkg, index) => {
            const accent = index % 3 === 1;
            const dark = index % 3 === 2;
            return (
              <Card
                key={pkg.id}
                className={cn(
                  "group relative min-h-[250px] overflow-hidden rounded-2xl border p-0 shadow-none transition-shadow duration-200 hover:shadow-md motion-reduce:transition-none",
                  accent
                    ? "border-workspace-accent bg-workspace-accent text-black"
                    : dark
                      ? "border-foreground bg-foreground text-background"
                      : "border-workspace-accent/20 bg-workspace-accent/5 text-foreground",
                )}
              >
                <PackageArtwork variant={index % 3} />
                <CardContent className="relative z-10 flex h-full flex-1 flex-col p-5">
                  <h3 className="text-sm font-medium opacity-80">{pkg.name}</h3>
                  <div className="mt-5 flex items-center gap-2">
                    <CoinIcon className="size-5" />
                    <span className="text-lg font-semibold tabular-nums">
                      {fmt.coinAmount(pkg.coin_amount)} coins
                    </span>
                  </div>
                  <p className="mt-4 text-3xl font-semibold tracking-tight tabular-nums sm:text-[clamp(1.25rem,2.2vw,1.875rem)]">
                    {fmt.vndAmount(pkg.price_vnd)}
                    <span className="ml-1 text-sm font-medium">VND</span>
                  </p>
                  <div className="mt-auto pt-6">
                    <Button
                      variant={accent || dark ? "secondary" : "default"}
                      disabled={isCheckingOut}
                      onClick={() => onCheckout(pkg)}
                      className="h-11 w-full justify-between rounded-xl"
                      aria-label={`Buy ${pkg.name} with PayOS`}
                    >
                      {isCheckingOut && checkingOutPackageId === pkg.id ? (
                        <>
                          Preparing PayOS…
                          <Loader2
                            className="size-4 animate-spin motion-reduce:animate-none"
                            aria-hidden
                          />
                        </>
                      ) : (
                        <>
                          Continue
                          <ArrowUpRight className="size-4" aria-hidden />
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

// Decorative geometry inspired by the supplied pricing reference; never an interactive control.
function PackageArtwork({ variant }: { variant: number }) {
  return (
    <svg
      viewBox="0 0 320 384"
      fill="currentColor"
      aria-hidden="true"
      className="pointer-events-none absolute -right-16 top-4 h-full w-80 opacity-[0.08] transition-transform duration-300 group-hover:-translate-y-2 group-focus-within:-translate-y-2 motion-reduce:transform-none motion-reduce:transition-none"
    >
      {variant === 0 ? (
        <>
          <circle cx="160" cy="110" r="100" />
          <ellipse cx="160" cy="270" rx="100" ry="44" />
        </>
      ) : variant === 1 ? (
        <>
          <rect
            x="40"
            y="30"
            width="150"
            height="150"
            rx="20"
            transform="rotate(15 115 105)"
          />
          <rect
            x="110"
            y="210"
            width="150"
            height="150"
            rx="20"
            transform="rotate(-15 185 285)"
          />
        </>
      ) : (
        <>
          {[0, 65, 130].map((y) => (
            <path
              key={y}
              transform={`translate(0 ${y})`}
              d="M160 35 270 145 Q280 155 270 165 L190 245 Q160 270 130 245 L50 165 Q40 155 50 145Z"
            />
          ))}
        </>
      )}
    </svg>
  );
}
