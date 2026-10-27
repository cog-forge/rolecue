"use client";

import { useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { gooeyToast } from "goey-toast";
import { Coins, LoaderCircle, Minus, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { getWallet, walletKeys } from "@/features/billing/api/wallet";
import { AddCoinsDialog } from "@/features/billing/components/wallet-purchase";
import { avatarKeys, inventoryFailure, purchaseSlots } from "../api/inventory";
import {
  maxSlotsPerPurchase,
  purchaseFormSchema,
  slotPriceCoins,
  type Capacity,
  type PurchaseForm,
} from "../schemas/avatar-schema";

const failureMessages: Record<string, string> = {
  INSUFFICIENT_BALANCE:
    "You don't have enough coins for this purchase. Top up your wallet, then try again.",
  VALIDATION_ERROR: `Choose between 1 and ${maxSlotsPerPurchase} slots.`,
};

const coins = (amount: number) =>
  `${amount} ${amount === 1 ? "coin" : "coins"}`;

export function BuySlotsDialog({
  open,
  onOpenChange,
  capacity,
  userId,
  role,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  capacity: Capacity | undefined;
  userId: string;
  role: "candidate" | "recruiter";
}) {
  const cache = useQueryClient();
  const [addingCoins, setAddingCoins] = useState(false);
  const wallet = useQuery({
    queryKey: walletKeys.wallet(userId),
    queryFn: ({ signal }) => getWallet(signal),
    enabled: open,
    refetchOnMount: "always",
    retry: false,
  });
  const purchase = useMutation({
    mutationFn: (values: PurchaseForm) => purchaseSlots(values.amount),
    onSuccess: (result, values) => {
      cache.setQueryData(avatarKeys.summary(userId), {
        capacity: result.capacity,
        used: result.used,
        available: result.available,
        is_full: result.is_full,
      });
      void cache.invalidateQueries({ queryKey: walletKeys.all(userId) });
      gooeyToast.success(
        `Added ${values.amount} avatar ${values.amount === 1 ? "slot" : "slots"}`,
        { description: "Your inventory capacity has been updated." },
      );
      close();
    },
    onError: (error) => {
      const failure = inventoryFailure(error, "purchase");
      if (failure.denied)
        void cache.invalidateQueries({ queryKey: ["auth", "me"] });
      // No idempotency key exists: show what the server holds now and let the user decide.
      if (!failure.confirmed) {
        void cache.invalidateQueries({ queryKey: avatarKeys.summary(userId) });
        void cache.invalidateQueries({ queryKey: walletKeys.all(userId) });
      }
    },
  });
  const pending = purchase.isPending;

  function close() {
    purchase.reset();
    onOpenChange(false);
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next && !pending) close();
      }}
    >
      <DialogContent className="sm:max-w-md">
        {/* Content unmounts while closed, so each opening starts from a fresh form. */}
        <PurchaseFields
          capacity={capacity}
          balance={
            wallet.isPending
              ? "pending"
              : wallet.isError
                ? "unavailable"
                : wallet.data.balance
          }
          pending={pending}
          error={purchase.error}
          onAddCoins={() => setAddingCoins(true)}
          onCancel={close}
          onSubmit={(values) => purchase.mutate(values)}
        />
      </DialogContent>
      <AddCoinsDialog
        open={addingCoins}
        onOpenChange={setAddingCoins}
        userId={userId}
        role={role}
      />
    </Dialog>
  );
}

function PurchaseFields({
  capacity,
  balance,
  pending,
  error,
  onAddCoins,
  onCancel,
  onSubmit,
}: {
  capacity: Capacity | undefined;
  balance: number | "pending" | "unavailable";
  pending: boolean;
  error: Error | null;
  onAddCoins: () => void;
  onCancel: () => void;
  onSubmit: (values: PurchaseForm) => void;
}) {
  const {
    register,
    control,
    setValue,
    handleSubmit,
    formState: { errors },
  } = useForm<PurchaseForm>({
    resolver: zodResolver(purchaseFormSchema),
    defaultValues: { amount: 1 },
  });
  const amount = useWatch({ control, name: "amount" });
  const valid = Number.isInteger(amount) && amount >= 1;
  const total = valid ? amount * slotPriceCoins : null;
  const missing =
    total !== null && typeof balance === "number"
      ? Math.max(total - balance, 0)
      : 0;
  const failure = error ? inventoryFailure(error, "purchase") : null;
  const message = !failure
    ? null
    : failure.confirmed
      ? failure.denied
        ? "Your session can't buy slots right now. Sign in again and retry."
        : (failureMessages[failure.code] ?? "We couldn't buy slots.")
      : "We couldn't confirm the purchase. Your capacity and balance were refreshed; check them before buying again.";
  const step = (delta: number) =>
    setValue(
      "amount",
      Math.min(maxSlotsPerPurchase, Math.max(1, (valid ? amount : 1) + delta)),
      { shouldValidate: true },
    );

  return (
    <form noValidate className="space-y-6" onSubmit={handleSubmit(onSubmit)}>
      <DialogHeader>
        <DialogTitle>Buy avatar slots</DialogTitle>
        <DialogDescription>
          Slots add storage space to your avatar inventory. Each slot costs{" "}
          {coins(slotPriceCoins)} and is paid from your wallet.
        </DialogDescription>
      </DialogHeader>
      <div className="space-y-2">
        <label htmlFor="slot-amount" className="text-sm font-medium">
          Number of slots
        </label>
        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="icon"
            className="size-11"
            aria-label="One slot fewer"
            disabled={pending || !valid || amount <= 1}
            onClick={() => step(-1)}
          >
            <Minus aria-hidden />
          </Button>
          <Input
            id="slot-amount"
            type="number"
            inputMode="numeric"
            min={1}
            max={maxSlotsPerPurchase}
            className="h-11 w-20 text-center tabular-nums"
            disabled={pending}
            aria-invalid={!!errors.amount}
            aria-describedby={
              errors.amount ? "slot-amount-error" : "slot-amount-hint"
            }
            {...register("amount", { valueAsNumber: true })}
          />
          <Button
            type="button"
            variant="outline"
            size="icon"
            className="size-11"
            aria-label="One slot more"
            disabled={pending || (valid && amount >= maxSlotsPerPurchase)}
            onClick={() => step(1)}
          >
            <Plus aria-hidden />
          </Button>
        </div>
        {errors.amount ? (
          <p
            id="slot-amount-error"
            role="alert"
            className="text-sm text-destructive"
          >
            {errors.amount.message}
          </p>
        ) : (
          <p id="slot-amount-hint" className="text-xs text-muted-foreground">
            1 to {maxSlotsPerPurchase} slots per purchase.
          </p>
        )}
      </div>
      <dl className="space-y-3 rounded-xl border bg-muted/40 p-4 text-sm">
        <div className="flex justify-between gap-4">
          <dt className="text-muted-foreground">Total</dt>
          <dd className="flex items-center gap-1.5 font-semibold tabular-nums">
            <Coins aria-hidden className="size-4 text-muted-foreground" />
            {total === null ? "–" : coins(total)}
          </dd>
        </div>
        <div className="flex justify-between gap-4">
          <dt className="text-muted-foreground">Your balance</dt>
          <dd className="tabular-nums" aria-live="polite">
            {balance === "pending"
              ? "Checking…"
              : balance === "unavailable"
                ? "Unavailable"
                : coins(balance)}
          </dd>
        </div>
        {capacity && valid && (
          <div className="flex justify-between gap-4">
            <dt className="text-muted-foreground">Capacity after</dt>
            <dd className="tabular-nums">{capacity.capacity + amount} slots</dd>
          </div>
        )}
      </dl>
      {missing > 0 && !message && (
        <p role="status" className="text-sm">
          You need {coins(missing)} more.{" "}
          <AddCoinsButton onClick={onAddCoins} />
        </p>
      )}
      {message && failure && (
        <div role="alert" className="space-y-1 text-sm">
          <p className={failure.confirmed ? "text-destructive" : undefined}>
            {message}
          </p>
          {failure.confirmed && failure.code === "INSUFFICIENT_BALANCE" && (
            <AddCoinsButton onClick={onAddCoins} />
          )}
        </div>
      )}
      <DialogFooter>
        <Button
          type="button"
          variant="outline"
          className="h-11"
          disabled={pending}
          onClick={onCancel}
        >
          Cancel
        </Button>
        <Button
          type="submit"
          className="h-11"
          disabled={pending || missing > 0}
          aria-busy={pending}
        >
          {pending && (
            <LoaderCircle
              aria-hidden
              className="size-4 animate-spin motion-reduce:animate-none"
            />
          )}
          {pending
            ? "Buying…"
            : total === null
              ? "Buy slots"
              : `Buy for ${coins(total)}`}
        </Button>
      </DialogFooter>
    </form>
  );
}

function AddCoinsButton({ onClick }: { onClick: () => void }) {
  return (
    <Button
      type="button"
      variant="link"
      className="h-auto p-0 font-medium underline underline-offset-4"
      onClick={onClick}
    >
      Add coins
    </Button>
  );
}
