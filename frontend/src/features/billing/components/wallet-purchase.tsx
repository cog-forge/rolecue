"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CoinIcon } from "@/components/icons/coin-icon";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogTrigger,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import {
  Tooltip,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";
import { Skeleton } from "@/components/ui/skeleton";
import type { ProductRole } from "@/config/permissions";
import { routes } from "@/config/routes";
import {
  getWallet,
  getCoinPackages,
  createDepositCheckout,
  walletKeys,
} from "../api/wallet";
import type { CoinPackage } from "../schemas/wallet-schema";
import { PackageGrid } from "./package-grid";

export function WalletPurchase({
  userId,
  role,
  collapsed,
}: {
  userId: string;
  role: ProductRole;
  collapsed: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [previewOpen, setPreviewOpen] = useState(false);
  const wallet = useQuery({
    queryKey: walletKeys.wallet(userId),
    queryFn: ({ signal }) => getWallet(signal),
    enabled: role !== "admin",
    staleTime: 30_000,
    refetchOnWindowFocus: true,
  });
  if (role === "admin") return null;
  const balance = wallet.data?.balance.toLocaleString("en-US");
  return (
    <AddCoinsDialog
      open={open}
      onOpenChange={setOpen}
      userId={userId}
      role={role}
    >
      <Tooltip open={!open && previewOpen} onOpenChange={setPreviewOpen}>
        <TooltipTrigger asChild>
          <DialogTrigger asChild>
            <Button
              variant="ghost"
              aria-label="Buy coins"
              className={
                collapsed
                  ? "h-auto min-h-11 w-full flex-col gap-1 rounded-xl px-1 py-2"
                  : "h-12 w-full justify-start gap-2.5 rounded-xl px-3"
              }
            >
              <CoinIcon className="size-6 shrink-0" />
              {wallet.isLoading ? (
                <Skeleton className="h-5 w-10" />
              ) : (
                <span className="max-w-full truncate text-base font-semibold tabular-nums text-amber-700 dark:text-amber-300">
                  {balance ?? "—"}
                </span>
              )}
            </Button>
          </DialogTrigger>
        </TooltipTrigger>
        <TooltipContent
          side="right"
          sideOffset={8}
          className="z-110 whitespace-nowrap duration-(--duration-quick) ease-out data-[state=closed]:animate-none motion-reduce:animate-none"
        >
          {wallet.isError ? "Balance unavailable · Add coins" : "Add coins"}
        </TooltipContent>
      </Tooltip>
    </AddCoinsDialog>
  );
}

/**
 * The coin package picker and PayOS checkout. The sidebar balance opens it; other screens that
 * need coins (buying avatar slots, the transactions page) open the same dialog instead of linking
 * away. PayOS returns to the billing page, which confirms the payment, unless the caller passes a
 * returnPath: a same-origin path (with its own query) that confirms the payment itself with
 * PaymentStatusBanner, such as the interview preflight.
 */
export function AddCoinsDialog({
  open,
  onOpenChange,
  userId,
  role,
  returnPath,
  children,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  userId: string;
  role: "candidate" | "recruiter";
  returnPath?: string;
  children?: React.ReactNode;
}) {
  const cache = useQueryClient();
  const packages = useQuery({
    queryKey: walletKeys.packages(),
    queryFn: ({ signal }) => getCoinPackages(signal),
    enabled: open,
    staleTime: 60_000,
  });
  const checkout = useMutation({
    mutationFn: (pkg: CoinPackage) => {
      const path =
        returnPath && isSameOriginPath(returnPath)
          ? returnPath
          : role === "recruiter"
            ? routes.recruiter.billing
            : routes.billing;
      return createDepositCheckout({
        package_id: pkg.id,
        return_url: paymentReturnUrl(path, "status", "success"),
        cancel_url: paymentReturnUrl(path, "cancel", "true"),
      });
    },
    onSuccess: (data) => {
      window.location.href = data.checkout_url;
    },
  });
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        onOpenChange(next);
        if (next) {
          if (!checkout.isPending) checkout.reset();
          void cache.invalidateQueries({ queryKey: walletKeys.wallet(userId) });
        }
      }}
    >
      {children}
      <DialogContent
        overlayClassName="z-100 motion-reduce:animate-none"
        className="z-100 flex max-h-[85dvh] w-[calc(100vw-2rem)] max-w-[840px] flex-col gap-0 overflow-hidden rounded-2xl p-0 sm:max-w-[840px] motion-reduce:animate-none [&>[data-slot=dialog-close]]:top-4 [&>[data-slot=dialog-close]]:right-4 [&>[data-slot=dialog-close]]:size-11"
      >
        <DialogHeader className="shrink-0 px-6 pt-6 pb-5 pr-16">
          <DialogTitle className="text-xl font-semibold tracking-tight">
            Add coins
          </DialogTitle>
          <DialogDescription>One-time payment · PayOS</DialogDescription>
        </DialogHeader>
        <div className="min-h-0 overflow-y-auto overscroll-contain px-6 pb-6">
          <PackageGrid
            packages={packages.data}
            isLoading={packages.isLoading}
            isFetching={packages.isFetching}
            error={packages.error}
            onRefresh={() => void packages.refetch()}
            onCheckout={(pkg) => checkout.mutate(pkg)}
            isCheckingOut={checkout.isPending}
            checkingOutPackageId={checkout.variables?.id}
            checkoutError={checkout.error?.message}
          />
        </div>
      </DialogContent>
    </Dialog>
  );
}

// A relative path on this origin only: "//host" and "/\\host" would leave the site.
function isSameOriginPath(path: string) {
  return /^\/(?![/\\])/.test(path);
}

// Keeps the path's own query (the interview setup choices) and adds PayOS's marker to it.
function paymentReturnUrl(path: string, name: string, value: string) {
  const url = new URL(path, window.location.origin);
  url.searchParams.set(name, value);
  return url.toString();
}
