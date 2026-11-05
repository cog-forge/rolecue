"use client";

import { useMemo, useRef, useState } from "react";
import { useTranslations } from "next-intl";
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
import { useFormatters } from "@/lib/i18n/use-formatters";
import { avatarKeys, inventoryFailure, purchaseSlots } from "../api/inventory";
import {
  maxSlotsPerPurchase,
  createPurchaseFormSchema,
  slotPriceCoins,
  type Capacity,
  type PurchaseForm,
} from "../schemas/avatar-schema";

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
  const t = useTranslations("candidate.avatars.buy");
  const cache = useQueryClient();
  const [addingCoins, setAddingCoins] = useState(false);
  const wallet = useQuery({
    queryKey: walletKeys.wallet(userId),
    queryFn: ({ signal }) => getWallet(signal),
    enabled: open,
    refetchOnMount: "always",
    retry: false,
  });
  // One key per purchase attempt: a retry after a lost response reuses it, so the API charges once.
  const attempt = useRef<{ amount: number; key: string } | null>(null);
  const purchase = useMutation({
    mutationFn: (values: PurchaseForm) => {
      if (attempt.current?.amount !== values.amount)
        attempt.current = { amount: values.amount, key: crypto.randomUUID() };
      return purchaseSlots(values.amount, attempt.current.key);
    },
    onSuccess: (result, values) => {
      attempt.current = null;
      cache.setQueryData(avatarKeys.summary(userId), {
        capacity: result.capacity,
        used: result.used,
        available: result.available,
        is_full: result.is_full,
      });
      void cache.invalidateQueries({ queryKey: walletKeys.all(userId) });
      gooeyToast.success(t("toastTitle", { count: values.amount }), {
        description: t("toastDescription"),
      });
      close();
    },
    onError: (error) => {
      const failure = inventoryFailure(error, "purchase");
      if (failure.denied)
        void cache.invalidateQueries({ queryKey: ["auth", "me"] });
      // A confirmed refusal charged nothing, so the next try is a new purchase. An unconfirmed one may
      // have charged: show what the server holds now; a retry keeps the key and cannot charge twice.
      if (failure.confirmed) attempt.current = null;
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
  const t = useTranslations("candidate.avatars.buy");
  const v = useTranslations("candidate.avatars.validation");
  const fmt = useFormatters();
  const purchaseSchema = useMemo(() => createPurchaseFormSchema(v), [v]);
  const {
    register,
    control,
    setValue,
    handleSubmit,
    formState: { errors },
  } = useForm<PurchaseForm>({
    resolver: zodResolver(purchaseSchema),
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
        ? t("errors.denied")
        : failure.code === "INSUFFICIENT_BALANCE"
          ? t("errors.insufficientBalance")
          : failure.code === "VALIDATION_ERROR"
            ? t("errors.validation", { max: maxSlotsPerPurchase })
            : t("errors.failed")
      : t("errors.unconfirmed");
  const step = (delta: number) =>
    setValue(
      "amount",
      Math.min(maxSlotsPerPurchase, Math.max(1, (valid ? amount : 1) + delta)),
      { shouldValidate: true },
    );

  return (
    <form noValidate className="space-y-6" onSubmit={handleSubmit(onSubmit)}>
      <DialogHeader>
        <DialogTitle>{t("title")}</DialogTitle>
        <DialogDescription>
          {t("description", { price: fmt.coins(slotPriceCoins) })}
        </DialogDescription>
      </DialogHeader>
      <div className="space-y-2">
        <label htmlFor="slot-amount" className="text-sm font-medium">
          {t("amountLabel")}
        </label>
        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="icon"
            className="size-11"
            aria-label={t("fewer")}
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
            aria-label={t("more")}
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
            {t("hint", { max: maxSlotsPerPurchase })}
          </p>
        )}
      </div>
      <dl className="space-y-3 rounded-xl border bg-muted/40 p-4 text-sm">
        <div className="flex justify-between gap-4">
          <dt className="text-muted-foreground">{t("total")}</dt>
          <dd className="flex items-center gap-1.5 font-semibold tabular-nums">
            <Coins aria-hidden className="size-4 text-muted-foreground" />
            {total === null ? "–" : fmt.coins(total)}
          </dd>
        </div>
        <div className="flex justify-between gap-4">
          <dt className="text-muted-foreground">{t("balance")}</dt>
          <dd className="tabular-nums" aria-live="polite">
            {balance === "pending"
              ? t("balanceChecking")
              : balance === "unavailable"
                ? t("balanceUnavailable")
                : fmt.coins(balance)}
          </dd>
        </div>
        {capacity && valid && (
          <div className="flex justify-between gap-4">
            <dt className="text-muted-foreground">{t("capacityAfter")}</dt>
            <dd className="tabular-nums">
              {t("capacityAfterValue", {
                count: capacity.capacity + amount,
                countText: String(capacity.capacity + amount),
              })}
            </dd>
          </div>
        )}
      </dl>
      {missing > 0 && !message && (
        <p role="status" className="text-sm">
          {t.rich("needMore", {
            amount: fmt.coins(missing),
            addCoins: (chunks) => (
              <AddCoinsButton onClick={onAddCoins}>{chunks}</AddCoinsButton>
            ),
          })}
        </p>
      )}
      {message && failure && (
        <div role="alert" className="space-y-1 text-sm">
          <p className={failure.confirmed ? "text-destructive" : undefined}>
            {message}
          </p>
          {failure.confirmed && failure.code === "INSUFFICIENT_BALANCE" && (
            <AddCoinsButton onClick={onAddCoins}>
              {t("addCoins")}
            </AddCoinsButton>
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
          {t("cancel")}
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
            ? t("pending")
            : total === null
              ? t("submitEmpty")
              : t("submit", { price: fmt.coins(total) })}
        </Button>
      </DialogFooter>
    </form>
  );
}

function AddCoinsButton({
  onClick,
  children,
}: {
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <Button
      type="button"
      variant="link"
      className="h-auto p-0 font-medium underline underline-offset-4"
      onClick={onClick}
    >
      {children}
    </Button>
  );
}
