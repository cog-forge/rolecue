"use client";

import { useRef, useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { CircleCheck, LoaderCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { ErrorState } from "@/components/ui/error-state";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { avatarKeys, getInventory } from "@/features/avatar/api/inventory";
import { BuySlotsDialog } from "@/features/avatar/components/buy-slots-dialog";
import { getWallet, walletKeys } from "@/features/billing/api/wallet";
import { AddCoinsDialog } from "@/features/billing/components/wallet-purchase";
import { jobPostingKeys } from "@/features/recruiter/job-postings/api/job-postings";
import { readApiError } from "@/lib/i18n/api-errors";
import { useApiErrorMessage } from "@/lib/i18n/use-api-error-message";
import { useFormatters } from "@/lib/i18n/use-formatters";
import {
  createPurchaseIntent,
  marketplaceKeys,
  purchaseListing,
  purchaseOutcomeUnknown,
} from "../api/marketplace";
import type { CatalogItem } from "../schemas/marketplace-schema";
import { CoinPrice } from "./marketplace-shared";

export function CheckoutDialog({
  item,
  onClose,
  onReload,
}: {
  item: CatalogItem;
  onClose: () => void;
  onReload: () => void;
}) {
  const user = useSessionUser();
  if (user.role === "admin") return null;
  return (
    <PersonalCheckout
      item={item}
      userId={user.id}
      role={user.role}
      onClose={onClose}
      onReload={onReload}
    />
  );
}
function PersonalCheckout({
  item,
  userId,
  role,
  onClose,
  onReload,
}: {
  item: CatalogItem;
  userId: string;
  role: "candidate" | "recruiter";
  onClose: () => void;
  onReload: () => void;
}) {
  const t = useTranslations("marketplace");
  const fmt = useFormatters();
  const message = useApiErrorMessage();
  const cache = useQueryClient();
  // Snapshot both the price and key once per mounted confirmation. Never replace them on retry/refetch.
  const [intent] = useState(() =>
    createPurchaseIntent(item.id, item.price_coins),
  );
  const busy = useRef(false);
  const [coinsOpen, setCoinsOpen] = useState(false);
  const [slotsOpen, setSlotsOpen] = useState(false);
  const wallet = useQuery({
    queryKey: walletKeys.wallet(userId),
    queryFn: ({ signal }) => getWallet(signal),
    refetchOnMount: "always",
    retry: false,
  });
  const inventory = useQuery({
    queryKey: avatarKeys.summary(userId),
    queryFn: ({ signal }) => getInventory(signal),
    refetchOnMount: "always",
    retry: false,
  });
  const refresh = () => {
    void cache.invalidateQueries({ queryKey: avatarKeys.all(userId) });
    void cache.invalidateQueries({ queryKey: walletKeys.all(userId) });
    void cache.invalidateQueries({ queryKey: marketplaceKeys.all(userId) });
    if (role === "recruiter")
      void cache.invalidateQueries({
        queryKey: jobPostingKeys.avatars(userId),
      });
  };
  const purchase = useMutation({
    mutationFn: () => purchaseListing(item.id, intent),
    onSuccess: (result) => {
      cache.setQueryData(
        walletKeys.wallet(userId),
        (previous: Awaited<ReturnType<typeof getWallet>> | undefined) =>
          previous ? { ...previous, balance: result.balance } : previous,
      );
      refresh();
      onReload();
    },
    onError: () => refresh(),
    retry: false,
  });
  const code = readApiError(purchase.error).code;
  const unknown = purchase.isError && purchaseOutcomeUnknown(purchase.error);
  const insufficient =
    code === "INSUFFICIENT_BALANCE" ||
    (!!wallet.data && wallet.data.balance < intent.price);
  const full = code === "INVENTORY_FULL" || inventory.data?.is_full === true;
  const blocked =
    purchase.isError &&
    !unknown &&
    !["INSUFFICIENT_BALANCE", "INVENTORY_FULL"].includes(code ?? "");
  const pending = purchase.isPending;
  const receipt = purchase.data;
  const unavailable = wallet.isError || inventory.isError;
  const canPurchase =
    unknown ||
    (!blocked &&
      !unavailable &&
      !!wallet.data &&
      !!inventory.data &&
      !insufficient &&
      !full);
  const historyHref = role === "candidate" ? "/billing" : "/recruiter/billing";
  function confirm() {
    if (busy.current || !canPurchase) return;
    busy.current = true;
    purchase.mutate(undefined, {
      onSettled: () => {
        busy.current = false;
      },
    });
  }
  return (
    <>
      <Dialog
        open
        onOpenChange={(open) => {
          if (!open && !pending) onClose();
        }}
      >
        <DialogContent
          className="max-h-[90dvh] overflow-y-auto sm:max-w-lg"
          onEscapeKeyDown={(event) => {
            if (pending) event.preventDefault();
          }}
          onPointerDownOutside={(event) => {
            if (pending) event.preventDefault();
          }}
        >
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              {receipt && (
                <CircleCheck aria-hidden className="size-5 text-emerald-600" />
              )}
              {receipt
                ? t(receipt.replayed ? "purchaseReplay" : "purchaseSuccess")
                : t("checkoutTitle")}
            </DialogTitle>
            <DialogDescription>
              {receipt
                ? receipt.avatar === null
                  ? t("deletedReplay")
                  : t("receipt")
                : t("checkoutDescription")}
            </DialogDescription>
          </DialogHeader>
          {receipt ? (
            <>
              <dl className="space-y-4 rounded-xl border bg-muted/30 p-4 text-sm">
                <SummaryRow label={t("paid")}>
                  <CoinPrice amount={receipt.transaction.amount} />
                </SummaryRow>
                <SummaryRow label={t("remainingBalance")}>
                  {fmt.coins(receipt.balance)}
                </SummaryRow>
                <SummaryRow label={t("purchaseDate")}>
                  {fmt.date(receipt.transaction.created_at, "localDateTime")}
                </SummaryRow>
                <div className="space-y-1">
                  <dt className="text-muted-foreground">{t("receiptId")}</dt>
                  <dd className="break-all font-mono text-xs">
                    {receipt.transaction.id}
                  </dd>
                </div>
              </dl>
              <DialogFooter className="flex-wrap sm:justify-between">
                <Button asChild variant="outline" className="min-h-11">
                  <Link href={historyHref}>{t("history")}</Link>
                </Button>
                <Button asChild className="min-h-11">
                  <Link href="/avatar-studio">{t("inventory")}</Link>
                </Button>
              </DialogFooter>
            </>
          ) : (
            <>
              <div className="flex items-start justify-between gap-4 border-b pb-4">
                <p className="break-words font-medium">{item.title}</p>
                <CoinPrice amount={intent.price} className="shrink-0" />
              </div>
              <dl className="space-y-3 text-sm">
                <SummaryRow label={t("balance")}>
                  {wallet.isPending
                    ? t("checking")
                    : wallet.data
                      ? fmt.coins(wallet.data.balance)
                      : t("unavailable")}
                </SummaryRow>
                <SummaryRow label={t("balanceAfter")}>
                  {wallet.data
                    ? fmt.coins(Math.max(wallet.data.balance - intent.price, 0))
                    : t("unavailable")}
                </SummaryRow>
                <SummaryRow label={t("capacity")}>
                  {inventory.isPending
                    ? t("checking")
                    : inventory.data
                      ? fmt.number(inventory.data.available)
                      : t("unavailable")}
                </SummaryRow>
              </dl>
              {unavailable && !unknown && (
                <ErrorState
                  variant="compact"
                  description={message(wallet.error || inventory.error)}
                  onRetry={() => {
                    void wallet.refetch();
                    void inventory.refetch();
                  }}
                  isRetrying={wallet.isFetching || inventory.isFetching}
                />
              )}
              {purchase.isError && (
                <div
                  role="alert"
                  className="space-y-2 rounded-xl border border-destructive/25 bg-destructive/5 p-4 text-sm leading-relaxed"
                >
                  <p>
                    {unknown
                      ? t("unknownPurchase")
                      : code === "MARKETPLACE_PRICE_CHANGED"
                        ? t("priceChanged")
                        : message(purchase.error)}
                  </p>
                  {unknown && (
                    <p className="text-xs text-muted-foreground">
                      {t("leaveUnknown")}
                    </p>
                  )}
                </div>
              )}
              {!unknown && !blocked && (insufficient || full) && (
                <div className="space-y-3 rounded-xl border bg-muted/30 p-4 text-sm">
                  {insufficient && (
                    <div className="space-y-2">
                      <p>{t("insufficient")}</p>
                      <Button
                        variant="outline"
                        className="min-h-11"
                        disabled={pending}
                        onClick={() => setCoinsOpen(true)}
                      >
                        {t("addCoins")}
                      </Button>
                    </div>
                  )}
                  {full && (
                    <div className="space-y-2">
                      <p>{t("slotsRequired")}</p>
                      <div className="flex flex-wrap gap-2">
                        <Button
                          variant="outline"
                          className="min-h-11"
                          disabled={pending}
                          onClick={() => setSlotsOpen(true)}
                        >
                          {t("addSlots")}
                        </Button>
                        <Button asChild variant="ghost" className="min-h-11">
                          <Link href="/avatar-studio">
                            {t("manageInventory")}
                          </Link>
                        </Button>
                      </div>
                    </div>
                  )}
                </div>
              )}
              <p className="text-xs leading-relaxed text-muted-foreground">
                {t("coinsOnly")}
              </p>
              <DialogFooter>
                <Button
                  variant="outline"
                  className="min-h-11"
                  disabled={pending}
                  onClick={onClose}
                >
                  {t("cancel")}
                </Button>
                {blocked ? (
                  code === "INVALID_TOKEN" ? (
                    <Button asChild className="min-h-11">
                      <Link
                        href={`/login?returnTo=${encodeURIComponent(`/marketplace/${item.id}`)}`}
                      >
                        {t("signIn")}
                      </Link>
                    </Button>
                  ) : code === "MARKETPLACE_ALREADY_OWNED" ? (
                    <Button asChild className="min-h-11">
                      <Link href="/avatar-studio">{t("inventory")}</Link>
                    </Button>
                  ) : (
                    <Button
                      className="min-h-11"
                      onClick={() => {
                        onReload();
                        onClose();
                      }}
                    >
                      {t("reloadListing")}
                    </Button>
                  )
                ) : (
                  <Button
                    className="min-h-11"
                    disabled={pending || !canPurchase}
                    aria-busy={pending}
                    onClick={confirm}
                  >
                    {pending && (
                      <LoaderCircle
                        aria-hidden
                        className="size-4 motion-safe:animate-spin"
                      />
                    )}
                    {pending
                      ? t("purchasing")
                      : unknown
                        ? t("retry")
                        : t("confirmBuy", { price: fmt.coins(intent.price) })}
                  </Button>
                )}
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>
      <AddCoinsDialog
        open={coinsOpen}
        onOpenChange={(open) => {
          setCoinsOpen(open);
          if (!open) {
            void wallet.refetch();
            if (code === "INSUFFICIENT_BALANCE") purchase.reset();
          }
        }}
        userId={userId}
        role={role}
        returnPath={`/marketplace/${item.id}`}
      />
      <BuySlotsDialog
        open={slotsOpen}
        onOpenChange={(open) => {
          setSlotsOpen(open);
          if (!open) {
            void inventory.refetch();
            if (code === "INVENTORY_FULL") purchase.reset();
          }
        }}
        capacity={inventory.data}
        userId={userId}
        role={role}
      />
    </>
  );
}
function SummaryRow({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex items-baseline justify-between gap-4">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="text-right font-medium tabular-nums">{children}</dd>
    </div>
  );
}
