"use client";

import { useEffect, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import { CheckCircle2, XCircle, AlertCircle, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { Transaction } from "../schemas/wallet-schema";

export interface PaymentReturnEvent {
  orderCode: number;
}

interface PaymentStatusBannerProps {
  onPaymentReturn: (event: PaymentReturnEvent) => void;
  verification: {
    variables?: number;
    data?: Transaction;
    isPending: boolean;
    isError: boolean;
  };
}

export function PaymentStatusBanner({
  onPaymentReturn,
  verification,
}: PaymentStatusBannerProps) {
  const searchParams = useSearchParams();
  const [dismissedKey, setDismissedKey] = useState<string | null>(null);
  const handledKeyRef = useRef<string | null>(null);
  const orderCode =
    searchParams.get("orderCode") || searchParams.get("order_code");
  const parsedCode =
    orderCode && /^\d+$/.test(orderCode) ? Number(orderCode) : null;
  const validCode =
    parsedCode && Number.isSafeInteger(parsedCode) && parsedCode > 0
      ? parsedCode
      : null;
  const hasPaymentReturn = [
    "status",
    "code",
    "cancel",
    "orderCode",
    "order_code",
  ].some((key) => searchParams.has(key));
  const currentKey = searchParams.toString();

  useEffect(() => {
    if (!hasPaymentReturn || !validCode || handledKeyRef.current === currentKey)
      return;
    handledKeyRef.current = currentKey;
    onPaymentReturn({ orderCode: validCode });
  }, [currentKey, hasPaymentReturn, validCode, onPaymentReturn]);

  if (!hasPaymentReturn || dismissedKey === currentKey) return null;

  // Redirect parameters are untrusted. Only a matching backend result determines the outcome.
  const matchesRequest =
    validCode !== null && verification.variables === validCode;
  const checking =
    validCode !== null && (!matchesRequest || verification.isPending);
  const result =
    matchesRequest &&
    !verification.isPending &&
    !verification.isError &&
    verification.data?.payos_order_code === validCode
      ? verification.data
      : undefined;
  const isSuccess = result?.status === "success";
  const isCancelled = result?.status === "cancelled";
  const isPending = result?.status === "pending";

  const handleDismiss = () => {
    setDismissedKey(currentKey);
    if (typeof window !== "undefined") {
      try {
        const base =
          window.location.origin && window.location.origin !== "null"
            ? window.location.origin
            : "http://localhost";
        const url = new URL(window.location.href || "/", base);
        url.searchParams.delete("status");
        url.searchParams.delete("code");
        url.searchParams.delete("cancel");
        url.searchParams.delete("orderCode");
        url.searchParams.delete("order_code");
        url.searchParams.delete("checkout");
        url.searchParams.delete("id");
        window.history.replaceState(
          window.history.state,
          "",
          url.pathname + (url.search ? url.search : "") + url.hash,
        );
      } catch {
        // Safe fallback in non-standard window environments
      }
    }
  };

  if (checking || isPending) {
    return (
      <div
        role="status"
        aria-live="polite"
        className="flex items-start gap-3 rounded-xl border border-border bg-muted/40 p-4"
      >
        <AlertCircle className="mt-0.5 h-5 w-5 shrink-0 text-muted-foreground" />
        <div className="flex-1 space-y-1">
          <h4 className="text-sm font-semibold">
            {checking ? "Verifying Payment" : "Payment Pending"}
          </h4>
          <p className="text-sm text-muted-foreground">
            {checking
              ? "Checking your payment with the payment provider. Please wait."
              : "Your payment has not been confirmed yet. Coins have not been credited."}
          </p>
          {isPending && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => onPaymentReturn({ orderCode: validCode! })}
            >
              Check again
            </Button>
          )}
        </div>
        <Button
          variant="ghost"
          size="sm"
          onClick={handleDismiss}
          aria-label="Dismiss payment notification"
        >
          <X className="h-4 w-4" />
        </Button>
      </div>
    );
  }

  if (isSuccess) {
    return (
      <div
        role="alert"
        aria-live="polite"
        className="relative overflow-hidden rounded-xl border border-emerald-500/30 bg-emerald-500/10 p-4 text-emerald-900 shadow-sm dark:bg-emerald-950/30 dark:text-emerald-200 transition-all duration-300"
      >
        <div className="flex items-start gap-3">
          <div className="flex-shrink-0 mt-0.5 rounded-full bg-emerald-500/20 p-1 text-emerald-600 dark:text-emerald-400">
            <CheckCircle2 className="h-5 w-5 animate-in zoom-in-75 duration-300" />
          </div>
          <div className="flex-1 space-y-1">
            <h4 className="text-sm font-semibold tracking-tight text-emerald-900 dark:text-emerald-100">
              Payment Completed Successfully
            </h4>
            <p className="text-sm text-emerald-800/90 dark:text-emerald-300/90">
              {orderCode
                ? `Order #${orderCode} was processed. Your coin balance has been credited.`
                : "Your deposit was processed. Your coin balance has been credited."}
            </p>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={handleDismiss}
            aria-label="Dismiss payment notification"
            className="h-8 w-8 p-0 text-emerald-800 hover:bg-emerald-500/20 hover:text-emerald-950 dark:text-emerald-300 dark:hover:bg-emerald-500/20 dark:hover:text-emerald-100"
          >
            <X className="h-4 w-4" />
          </Button>
        </div>
      </div>
    );
  }

  if (isCancelled) {
    return (
      <div
        role="alert"
        aria-live="polite"
        className="relative overflow-hidden rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-amber-900 shadow-sm dark:bg-amber-950/30 dark:text-amber-200 transition-all duration-300"
      >
        <div className="flex items-start gap-3">
          <div className="flex-shrink-0 mt-0.5 rounded-full bg-amber-500/20 p-1 text-amber-600 dark:text-amber-400">
            <AlertCircle className="h-5 w-5" />
          </div>
          <div className="flex-1 space-y-1">
            <h4 className="text-sm font-semibold tracking-tight text-amber-900 dark:text-amber-100">
              Payment Cancelled
            </h4>
            <p className="text-sm text-amber-800/90 dark:text-amber-300/90">
              {orderCode
                ? `Payment for Order #${orderCode} was cancelled. No coins were credited.`
                : "Payment was cancelled. No coins were credited."}
            </p>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={handleDismiss}
            aria-label="Dismiss payment notification"
            className="h-8 w-8 p-0 text-amber-800 hover:bg-amber-500/20 hover:text-amber-950 dark:text-amber-300 dark:hover:bg-amber-500/20 dark:hover:text-amber-100"
          >
            <X className="h-4 w-4" />
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div
      role="alert"
      aria-live="assertive"
      className="relative overflow-hidden rounded-xl border border-destructive/30 bg-destructive/10 p-4 text-destructive shadow-sm dark:bg-destructive/15 transition-all duration-300"
    >
      <div className="flex items-start gap-3">
        <div className="flex-shrink-0 mt-0.5 rounded-full bg-destructive/20 p-1 text-destructive">
          <XCircle className="h-5 w-5" />
        </div>
        <div className="flex-1 space-y-1">
          <h4 className="text-sm font-semibold tracking-tight">
            {result?.status === "failed"
              ? "Payment Unsuccessful"
              : "Unable to Verify Payment"}
          </h4>
          <p className="text-sm opacity-90">
            {orderCode
              ? `We could not verify payment for Order #${orderCode}. If you were charged, please contact support with your order code.`
              : "We could not verify your payment. Return with a valid order code or contact support if you were charged."}
          </p>
          {validCode && !result && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => onPaymentReturn({ orderCode: validCode })}
            >
              Check again
            </Button>
          )}
        </div>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={handleDismiss}
          aria-label="Dismiss payment notification"
          className="h-8 w-8 p-0 text-destructive hover:bg-destructive/20"
        >
          <X className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}
