"use client";

import { useEffect, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import { CheckCircle2, XCircle, AlertCircle, X } from "lucide-react";
import { Button } from "@/components/ui/button";

export interface PaymentReturnEvent {
  type: "success" | "cancelled" | "failed";
  orderCode?: number | null;
}

interface PaymentStatusBannerProps {
  onPaymentSuccess?: () => void;
  onPaymentReturn?: (event: PaymentReturnEvent) => void;
}

export function PaymentStatusBanner({
  onPaymentSuccess,
  onPaymentReturn,
}: PaymentStatusBannerProps) {
  const searchParams = useSearchParams();
  const [dismissed, setDismissed] = useState(false);
  const handledKeyRef = useRef<string | null>(null);

  const status = searchParams.get("status");
  const code = searchParams.get("code");
  const cancel = searchParams.get("cancel");
  const orderCode =
    searchParams.get("orderCode") || searchParams.get("order_code");

  const isSuccess =
    status === "PAID" ||
    status === "success" ||
    (code === "00" && cancel !== "true");
  const isCancelled =
    cancel === "true" || status === "CANCELLED" || status === "cancelled";
  const isFailed = (status === "FAILED" || status === "failed") && !isCancelled;

  const hasPaymentReturn = isSuccess || isCancelled || isFailed;
  const currentKey = `${status}:${code}:${cancel}:${orderCode}`;

  useEffect(() => {
    if (!hasPaymentReturn) return;
    if (handledKeyRef.current === currentKey) return;
    handledKeyRef.current = currentKey;

    if (isSuccess && onPaymentSuccess) {
      onPaymentSuccess();
    }

    if (onPaymentReturn) {
      const parsed = orderCode ? parseInt(orderCode, 10) : null;
      const validCode = parsed && !isNaN(parsed) && parsed > 0 ? parsed : null;
      if (isSuccess) {
        onPaymentReturn({ type: "success", orderCode: validCode });
      } else if (isCancelled) {
        onPaymentReturn({ type: "cancelled", orderCode: validCode });
      } else if (isFailed) {
        onPaymentReturn({ type: "failed", orderCode: validCode });
      }
    }
  }, [
    currentKey,
    hasPaymentReturn,
    isSuccess,
    isCancelled,
    isFailed,
    orderCode,
    onPaymentSuccess,
    onPaymentReturn,
  ]);

  if (!hasPaymentReturn || dismissed) {
    return null;
  }

  const handleDismiss = () => {
    setDismissed(true);
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
                ? `Checkout for Order #${orderCode} was cancelled. No charges were made to your account.`
                : "Checkout session was cancelled. No charges were made to your account."}
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
            Payment Unsuccessful
          </h4>
          <p className="text-sm opacity-90">
            {orderCode
              ? `We could not verify payment for Order #${orderCode}. If you were charged, please contact support with your order code.`
              : "We could not verify your payment. Please try again or select another package."}
          </p>
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
