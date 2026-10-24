"use client";

import { useState } from "react";
import Link from "next/link";
import {
  CheckCircle2,
  Clock,
  XCircle,
  AlertCircle,
  ArrowDownLeft,
  ArrowUpRight,
  RotateCcw,
  Copy,
  Check,
  ExternalLink,
  RefreshCw,
  Ban,
  Receipt,
  AlertTriangle,
} from "lucide-react";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Checkbox } from "@/components/ui/checkbox";
import type { AdminTransactionItem } from "../schemas/billing-schema";
import { formatVND, formatDateTime, formatCoins } from "../lib/format";
import {
  syncPayOSTransaction,
  cancelTransaction,
  issueRefund,
} from "../api/billing";

// Legacy export for backward compatibility
export type AdminTransaction = AdminTransactionItem;

// ─── Status Badge ─────────────────────────────────────────────────────────────

export function TxStatusBadge({
  status,
  isStalePending = false,
}: {
  status: string;
  isStalePending?: boolean;
}) {
  switch (status.toLowerCase()) {
    case "success":
      return (
        <Badge
          variant="outline"
          className="gap-1 border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 text-[11px] font-medium"
        >
          <CheckCircle2 className="h-3 w-3" aria-hidden />
          Success
        </Badge>
      );
    case "pending":
      return (
        <Badge
          variant="outline"
          className={`gap-1 text-[11px] font-medium ${
            isStalePending
              ? "border-amber-600/50 bg-amber-500/20 text-amber-800 dark:text-amber-300 ring-1 ring-amber-500/30"
              : "border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-400"
          }`}
        >
          <Clock className="h-3 w-3" aria-hidden />
          {isStalePending ? "Pending (>4h)" : "Pending"}
        </Badge>
      );
    case "cancelled":
    case "expired":
      return (
        <Badge
          variant="outline"
          className="gap-1 border-muted-foreground/30 bg-muted/60 text-muted-foreground text-[11px] font-medium"
        >
          <AlertCircle className="h-3 w-3" aria-hidden />
          {status.charAt(0).toUpperCase() + status.slice(1)}
        </Badge>
      );
    case "refunded":
      return (
        <Badge
          variant="outline"
          className="gap-1 border-purple-500/30 bg-purple-500/10 text-purple-700 dark:text-purple-400 text-[11px] font-medium"
        >
          <RotateCcw className="h-3 w-3" aria-hidden />
          Refunded
        </Badge>
      );
    case "failed":
      return (
        <Badge
          variant="outline"
          className="gap-1 border-destructive/30 bg-destructive/10 text-destructive text-[11px] font-medium"
        >
          <XCircle className="h-3 w-3" aria-hidden />
          Failed
        </Badge>
      );
    default:
      return (
        <Badge variant="outline" className="text-[11px]">
          {status}
        </Badge>
      );
  }
}

// ─── Party & Type Helpers ─────────────────────────────────────────────────────

export function resolveParty(
  raw: string | null | undefined,
  role: "from" | "to",
  isTopup: boolean,
  isRefund: boolean,
): { label: string; isSystemOrBanking: boolean } {
  const isInvalid =
    !raw ||
    raw.trim() === "" ||
    raw === "00000000-0000-0000-0000-000000000000" ||
    raw === "null";

  if (isInvalid) {
    if (role === "from") {
      return {
        label: isTopup ? "Banking (PayOS)" : isRefund ? "System" : "User",
        isSystemOrBanking: true,
      };
    }
    return {
      label: isTopup || isRefund ? "User" : "System",
      isSystemOrBanking: true,
    };
  }

  const lower = raw.toLowerCase();
  if (lower.includes("banking") || lower.includes("payos")) {
    return { label: "Banking (PayOS)", isSystemOrBanking: true };
  }
  if (lower === "system") {
    return { label: "System", isSystemOrBanking: true };
  }

  return { label: raw, isSystemOrBanking: false };
}

function isTxDeposit(tx: AdminTransactionItem): boolean {
  if (tx.payos_order_code != null) return true;
  if (tx.type === "topup" && !tx.description?.toLowerCase().includes("charge"))
    return true;
  return false;
}

function isTxRefund(tx: AdminTransactionItem): boolean {
  return (
    tx.type === "refund" ||
    (tx.description?.toLowerCase().includes("refund") ?? false) ||
    tx.from === "System"
  );
}

function txMeta(tx: AdminTransactionItem) {
  const refund = isTxRefund(tx);
  const deposit = isTxDeposit(tx) && !refund;
  if (refund) return { label: "Refund", Icon: RotateCcw, isPositive: true };
  if (deposit)
    return { label: "Top-up", Icon: ArrowDownLeft, isPositive: true };
  return { label: "Spending", Icon: ArrowUpRight, isPositive: false };
}

function isPendingOlderThan(createdAt: string, hours: number): boolean {
  if (!createdAt) return false;
  const created = new Date(createdAt).getTime();
  const diffHours = (Date.now() - created) / (1000 * 60 * 60);
  return diffHours > hours;
}

// ─── Copy Button ──────────────────────────────────────────────────────────────

function CopyButton({
  text,
  label = "Copy",
}: {
  text: string;
  label?: string;
}) {
  const [copied, setCopied] = useState(false);

  function handleCopy(e: React.MouseEvent) {
    e.stopPropagation();
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }

  return (
    <Button
      type="button"
      size="icon"
      variant="ghost"
      className="h-6 w-6 text-muted-foreground hover:text-foreground"
      onClick={handleCopy}
      title={label}
    >
      {copied ? (
        <Check className="h-3 w-3 text-emerald-500" />
      ) : (
        <Copy className="h-3 w-3" />
      )}
    </Button>
  );
}

// ─── Detail Drawer (Sheet) ────────────────────────────────────────────────────

export function TransactionDetailDrawer({
  tx,
  onClose,
  onUpdated,
}: {
  tx: AdminTransactionItem | null;
  onClose: () => void;
  onUpdated?: () => void;
}) {
  const [isSyncing, setIsSyncing] = useState(false);
  const [isCancelling, setIsCancelling] = useState(false);
  const [cancelReason, setCancelReason] = useState("");
  const [showCancelDialog, setShowCancelDialog] = useState(false);
  const [showRefundDialog, setShowRefundDialog] = useState(false);
  const [refundAmount, setRefundAmount] = useState(
    tx?.paid_vnd ? String(tx.paid_vnd) : "50000",
  );
  const [refundReason, setRefundReason] = useState("");
  const [clawbackCoins, setClawbackCoins] = useState(true);
  const [actionError, setActionError] = useState<string | null>(null);

  if (!tx) return null;

  const { label, Icon, isPositive } = txMeta(tx);
  const deposit = isTxDeposit(tx);
  const refund = isTxRefund(tx);
  const fromParty = resolveParty(tx.from, "from", deposit, refund);
  const toParty = resolveParty(tx.to, "to", deposit, refund);
  const isStale =
    tx.status === "pending" && isPendingOlderThan(tx.created_at || "", 4);

  async function handleSync() {
    if (!tx) return;
    setIsSyncing(true);
    setActionError(null);
    try {
      await syncPayOSTransaction(tx.id);
      onUpdated?.();
      onClose();
    } catch (err: unknown) {
      setActionError(err instanceof Error ? err.message : "Sync failed");
    } finally {
      setIsSyncing(false);
    }
  }

  async function handleCancel() {
    if (!tx) return;
    setIsCancelling(true);
    setActionError(null);
    try {
      await cancelTransaction(
        tx.id,
        cancelReason || "Cancelled by administrator",
      );
      setShowCancelDialog(false);
      onUpdated?.();
      onClose();
    } catch (err: unknown) {
      setActionError(
        err instanceof Error ? err.message : "Cancellation failed",
      );
    } finally {
      setIsCancelling(false);
    }
  }

  async function handleRefund() {
    if (!tx) return;
    setActionError(null);
    try {
      const amountNum = parseInt(refundAmount, 10);
      if (isNaN(amountNum) || amountNum <= 0) {
        setActionError("Please enter a valid refund amount");
        return;
      }
      const userId =
        tx.user?.id || (toParty.isSystemOrBanking ? tx.to : tx.from);
      if (!userId) {
        setActionError("Cannot determine user ID for this transaction");
        return;
      }
      await issueRefund(tx.id, {
        user_id: userId,
        amount: amountNum,
        currency: tx.currency === "COIN" ? "COIN" : "VND",
        reason: refundReason || "Customer refund issued by admin",
        clawback_coins: clawbackCoins,
      });
      setShowRefundDialog(false);
      onUpdated?.();
      onClose();
    } catch (err: unknown) {
      setActionError(err instanceof Error ? err.message : "Refund failed");
    }
  }

  return (
    <>
      <Sheet open={true} onOpenChange={(open) => !open && onClose()}>
        <SheetContent className="w-full sm:max-w-lg overflow-y-auto p-0">
          <SheetHeader className="border-b border-border/80 p-6 bg-muted/20">
            <div className="flex items-start justify-between gap-3">
              <div className="flex items-center gap-3">
                <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary border border-primary/20">
                  <Icon className="h-5 w-5" aria-hidden />
                </span>
                <div>
                  <SheetTitle className="text-base font-semibold">
                    Transaction details
                  </SheetTitle>
                  <SheetDescription className="text-xs text-muted-foreground mt-0.5">
                    {tx.package_or_feature || label} · {label}
                  </SheetDescription>
                </div>
              </div>
              <TxStatusBadge status={tx.status} isStalePending={isStale} />
            </div>
          </SheetHeader>

          <div className="p-6 space-y-6">
            {actionError && (
              <div className="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-xs text-destructive flex items-center gap-2">
                <AlertCircle className="h-4 w-4 shrink-0" />
                <span>{actionError}</span>
              </div>
            )}

            {isStale && (
              <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 p-3.5 text-xs text-amber-800 dark:text-amber-300 space-y-2">
                <div className="flex items-center gap-2 font-medium">
                  <AlertTriangle className="h-4 w-4 text-amber-600 dark:text-amber-400 shrink-0" />
                  <span>Pending payment older than 4 hours</span>
                </div>
                <p className="text-[11px] leading-relaxed text-muted-foreground">
                  The user may have abandoned the PayOS checkout or the webhook
                  was delayed. You can re-check with PayOS or mark it as
                  cancelled.
                </p>
              </div>
            )}

            {/* Financial Overview Card */}
            <div className="rounded-xl border border-border/80 bg-card p-4 space-y-3">
              <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Amount & Currency
              </p>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <p className="text-xs text-muted-foreground">Paid (VND)</p>
                  <p className="text-lg font-bold text-foreground tabular-nums mt-0.5">
                    {tx.paid_vnd != null ? formatVND(tx.paid_vnd) : "—"}
                  </p>
                </div>
                <div>
                  <p className="text-xs text-muted-foreground">Coins</p>
                  <p className="text-lg font-bold text-foreground tabular-nums mt-0.5">
                    {tx.coins != null ? (
                      <span
                        className={
                          isPositive
                            ? "text-emerald-600 dark:text-emerald-400"
                            : ""
                        }
                      >
                        {isPositive ? "+" : "-"}
                        {formatCoins(Math.abs(tx.coins))}
                      </span>
                    ) : (
                      "—"
                    )}
                  </p>
                </div>
              </div>

              {/* Balance Before & After */}
              {(tx.balance_before != null || tx.balance_after != null) && (
                <div className="pt-2 border-t border-border/50 text-xs flex items-center justify-between text-muted-foreground">
                  <span>Balance transition:</span>
                  <span className="font-mono font-medium text-foreground">
                    {tx.balance_before ?? 0} coins → {tx.balance_after ?? 0}{" "}
                    coins
                  </span>
                </div>
              )}
            </div>

            {/* Parties & Counterparts */}
            <div className="rounded-xl border border-border/80 bg-card p-4 space-y-3">
              <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Counterparties
              </p>
              <dl className="space-y-2 text-xs">
                <div className="flex items-start justify-between gap-4 py-1 border-b border-border/40">
                  <dt className="text-muted-foreground font-medium">From</dt>
                  <dd className="text-right font-medium text-foreground break-all">
                    {fromParty.label}
                  </dd>
                </div>
                <div className="flex items-start justify-between gap-4 py-1 border-b border-border/40">
                  <dt className="text-muted-foreground font-medium">To</dt>
                  <dd className="text-right font-medium text-foreground break-all">
                    {toParty.label}
                  </dd>
                </div>
                {tx.user && (
                  <div className="flex items-start justify-between gap-4 py-1">
                    <dt className="text-muted-foreground font-medium">
                      User Account
                    </dt>
                    <dd className="text-right">
                      <Link
                        href={`/admin/users?search=${encodeURIComponent(tx.user.email)}`}
                        className="font-medium text-primary hover:underline inline-flex items-center gap-1"
                      >
                        {tx.user.name || tx.user.email}
                        <ExternalLink className="h-3 w-3" />
                      </Link>
                      <p className="text-[11px] text-muted-foreground">
                        {tx.user.email}
                      </p>
                    </dd>
                  </div>
                )}
              </dl>
            </div>

            {/* Gateway & Identifiers */}
            <div className="rounded-xl border border-border/80 bg-card p-4 space-y-3">
              <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Gateway & Identifiers
              </p>
              <dl className="space-y-2.5 text-xs">
                <div className="flex items-center justify-between gap-2">
                  <dt className="text-muted-foreground">Transaction ID</dt>
                  <dd className="flex items-center gap-1 font-mono text-[11px] text-foreground">
                    <span className="truncate max-w-[200px]" title={tx.id}>
                      {tx.id}
                    </span>
                    <CopyButton text={tx.id} label="Copy Transaction ID" />
                  </dd>
                </div>

                {tx.payos_order_code != null && (
                  <div className="flex items-center justify-between gap-2">
                    <dt className="text-muted-foreground">PayOS Order Code</dt>
                    <dd className="flex items-center gap-1 font-mono text-xs font-semibold text-foreground">
                      #{tx.payos_order_code}
                      <CopyButton
                        text={String(tx.payos_order_code)}
                        label="Copy Order Code"
                      />
                    </dd>
                  </div>
                )}

                {tx.payos_payment_link_id && (
                  <div className="flex items-center justify-between gap-2">
                    <dt className="text-muted-foreground">Payment Link ID</dt>
                    <dd className="flex items-center gap-1 font-mono text-[11px] text-muted-foreground">
                      <span className="truncate max-w-[180px]">
                        {tx.payos_payment_link_id}
                      </span>
                      <CopyButton
                        text={tx.payos_payment_link_id}
                        label="Copy Payment Link ID"
                      />
                    </dd>
                  </div>
                )}

                {tx.failure_reason && (
                  <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-2.5 text-destructive space-y-1">
                    <p className="font-semibold text-xs flex items-center gap-1">
                      <XCircle className="h-3.5 w-3.5" /> Failure Reason
                    </p>
                    <p className="text-xs">{tx.failure_reason}</p>
                  </div>
                )}

                {tx.original_transaction_id && (
                  <div className="flex items-center justify-between gap-2 pt-1 border-t border-border/40">
                    <dt className="text-muted-foreground">
                      Original Transaction
                    </dt>
                    <dd className="flex items-center gap-1 font-mono text-xs text-primary">
                      <span className="truncate max-w-[160px]">
                        {tx.original_transaction_id}
                      </span>
                      <CopyButton
                        text={tx.original_transaction_id}
                        label="Copy Original TX ID"
                      />
                    </dd>
                  </div>
                )}

                {refund && (
                  <div className="flex items-center justify-between gap-2">
                    <dt className="text-muted-foreground">Coin Clawback</dt>
                    <dd className="text-xs font-medium">
                      {tx.coin_clawed_back ? (
                        <span className="text-emerald-600 dark:text-emerald-400">
                          Coins clawed back
                        </span>
                      ) : (
                        <span className="text-muted-foreground">
                          No coins clawed back
                        </span>
                      )}
                    </dd>
                  </div>
                )}

                {tx.is_test && (
                  <div className="flex items-center justify-between gap-2">
                    <dt className="text-muted-foreground">Environment</dt>
                    <dd>
                      <Badge
                        variant="outline"
                        className="text-[10px] bg-amber-500/10 text-amber-600 border-amber-500/20"
                      >
                        Test / Seed Row
                      </Badge>
                    </dd>
                  </div>
                )}
              </dl>
            </div>

            {/* Timestamps */}
            <div className="text-xs text-muted-foreground space-y-1 px-1">
              <div className="flex justify-between">
                <span>Created (UTC+7):</span>
                <span className="font-mono">
                  {formatDateTime(tx.created_at)}
                </span>
              </div>
              <div className="flex justify-between">
                <span>Updated (UTC+7):</span>
                <span className="font-mono">
                  {formatDateTime(tx.updated_at)}
                </span>
              </div>
            </div>

            {/* Admin Actions */}
            <div className="border-t border-border/80 pt-4 space-y-2">
              <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Admin Actions
              </p>
              <div className="flex flex-wrap gap-2">
                {tx.status === "pending" && (
                  <>
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      className="text-xs gap-1.5"
                      onClick={handleSync}
                      disabled={isSyncing}
                    >
                      <RefreshCw
                        className={`h-3.5 w-3.5 ${isSyncing ? "animate-spin" : ""}`}
                      />
                      Re-check with PayOS
                    </Button>
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      className="text-xs gap-1.5 text-destructive hover:bg-destructive/10 hover:text-destructive"
                      onClick={() => setShowCancelDialog(true)}
                    >
                      <Ban className="h-3.5 w-3.5" />
                      Mark Cancelled
                    </Button>
                  </>
                )}

                {tx.status === "success" && !refund && (
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    className="text-xs gap-1.5"
                    onClick={() => setShowRefundDialog(true)}
                  >
                    <RotateCcw className="h-3.5 w-3.5" />
                    Issue Refund
                  </Button>
                )}
              </div>
            </div>
          </div>
        </SheetContent>
      </Sheet>

      {/* Cancel Dialog */}
      <Dialog open={showCancelDialog} onOpenChange={setShowCancelDialog}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Cancel Pending Transaction</DialogTitle>
            <DialogDescription>
              Mark transaction #{tx.payos_order_code || tx.id.slice(0, 8)} as
              cancelled. This action will be recorded in the audit log.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3 py-2">
            <label
              htmlFor="cancel-reason"
              className="text-xs font-medium text-foreground"
            >
              Reason for cancellation
            </label>
            <Input
              id="cancel-reason"
              placeholder="e.g. Expired checkout session, customer abandoned order"
              value={cancelReason}
              onChange={(e) => setCancelReason(e.target.value)}
            />
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setShowCancelDialog(false)}
              disabled={isCancelling}
            >
              Back
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={handleCancel}
              disabled={isCancelling}
            >
              {isCancelling ? "Cancelling..." : "Confirm Cancellation"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Refund Dialog */}
      <Dialog open={showRefundDialog} onOpenChange={setShowRefundDialog}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Issue Refund / Coin Adjustment</DialogTitle>
            <DialogDescription>
              Issue a refund for transaction {tx.id.slice(0, 8)}. Net revenue
              will be deducted and an audit log recorded.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3 py-2 text-xs">
            <div>
              <label
                htmlFor="refund-amount"
                className="font-medium text-foreground"
              >
                Refund Amount (VND)
              </label>
              <Input
                id="refund-amount"
                type="number"
                className="mt-1"
                value={refundAmount}
                onChange={(e) => setRefundAmount(e.target.value)}
              />
            </div>
            <div>
              <label
                htmlFor="refund-reason"
                className="font-medium text-foreground"
              >
                Required Reason
              </label>
              <Input
                id="refund-reason"
                className="mt-1"
                placeholder="e.g. Service dissatisfaction, customer request"
                value={refundReason}
                onChange={(e) => setRefundReason(e.target.value)}
              />
            </div>
            <div className="flex items-center gap-2 pt-1">
              <Checkbox
                id="clawback-checkbox"
                checked={clawbackCoins}
                onCheckedChange={(c) => setClawbackCoins(!!c)}
              />
              <label
                htmlFor="clawback-checkbox"
                className="font-normal cursor-pointer"
              >
                Claw back user coin balance equivalent to this refund
              </label>
            </div>
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setShowRefundDialog(false)}
            >
              Cancel
            </Button>
            <Button type="button" onClick={handleRefund}>
              Confirm Refund
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

// ─── Table Component ──────────────────────────────────────────────────────────

export function TransactionTable({
  transactions,
  isLoading,
  selectedTx,
  onSelectTx,
  onDrawerClose,
  onTxUpdated,
}: {
  transactions: AdminTransactionItem[];
  isLoading: boolean;
  selectedTx?: AdminTransactionItem | null;
  onSelectTx: (tx: AdminTransactionItem) => void;
  onDrawerClose?: () => void;
  onTxUpdated?: () => void;
}) {
  if (isLoading) {
    return (
      <div className="space-y-2 p-4">
        {Array.from({ length: 6 }).map((_, i) => (
          <div
            key={i}
            className="flex items-center justify-between py-3 border-b border-border/40"
          >
            <div className="flex items-center gap-3">
              <Skeleton className="h-8 w-8 rounded-lg" />
              <div className="space-y-1.5">
                <Skeleton className="h-3 w-32" />
                <Skeleton className="h-4 w-48" />
              </div>
            </div>
            <Skeleton className="h-4 w-20" />
            <Skeleton className="h-5 w-16 rounded-full" />
            <Skeleton className="h-3 w-24" />
          </div>
        ))}
      </div>
    );
  }

  if (transactions.length === 0) {
    return (
      <div className="p-12 text-center">
        <Receipt className="mx-auto h-10 w-10 text-muted-foreground/50" />
        <h3 className="mt-3 text-sm font-semibold text-foreground">
          No transactions recorded
        </h3>
        <p className="mt-1 text-xs text-muted-foreground max-w-sm mx-auto">
          No records match your active filters or date selection. Try resetting
          filters or choosing a broader date preset.
        </p>
      </div>
    );
  }

  return (
    <>
      <div className="overflow-x-auto">
        <table className="w-full text-left text-sm border-collapse">
          <thead>
            <tr className="border-b border-border/80 bg-muted/30 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
              <th className="px-5 py-3.5">Transaction ID</th>
              <th className="px-5 py-3.5">User</th>
              <th className="px-5 py-3.5">Package / Feature</th>
              <th className="px-5 py-3.5 text-right">Paid (VND)</th>
              <th className="px-5 py-3.5 text-right">Coins</th>
              <th className="px-5 py-3.5">Status</th>
              <th className="px-5 py-3.5 text-right">Date (UTC+7)</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border/40">
            {transactions.map((tx) => {
              const { label, isPositive } = txMeta(tx);
              const deposit = isTxDeposit(tx);
              const refund = isTxRefund(tx);
              const fromParty = resolveParty(tx.from, "from", deposit, refund);
              const toParty = resolveParty(tx.to, "to", deposit, refund);
              const isStale =
                tx.status === "pending" &&
                isPendingOlderThan(tx.created_at || "", 4);

              return (
                <tr
                  key={tx.id}
                  onClick={() => onSelectTx(tx)}
                  className="group cursor-pointer hover:bg-muted/40 transition-colors duration-150"
                  tabIndex={0}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") onSelectTx(tx);
                  }}
                  role="button"
                  aria-label={`View transaction ${tx.id}`}
                >
                  {/* Transaction ID */}
                  <td className="px-5 py-3.5">
                    <div className="flex items-center gap-1 font-mono text-xs text-muted-foreground">
                      <span className="truncate max-w-[90px]" title={tx.id}>
                        {tx.id.slice(0, 8)}…
                      </span>
                      <CopyButton text={tx.id} label="Copy ID" />
                    </div>
                  </td>

                  {/* User */}
                  <td className="px-5 py-3.5">
                    {tx.user ? (
                      <div className="min-w-0">
                        <Link
                          href={`/admin/users?search=${encodeURIComponent(tx.user.email)}`}
                          onClick={(e) => e.stopPropagation()}
                          className="font-medium text-xs text-foreground hover:text-primary hover:underline truncate block max-w-[180px]"
                        >
                          {tx.user.name || tx.user.email}
                        </Link>
                        <p className="text-[11px] text-muted-foreground truncate max-w-[180px]">
                          {tx.user.email}
                        </p>
                      </div>
                    ) : (
                      <div className="text-xs">
                        {fromParty.isSystemOrBanking ? (
                          <span className="text-muted-foreground font-medium">
                            {fromParty.label}
                          </span>
                        ) : toParty.isSystemOrBanking ? (
                          <span className="text-muted-foreground font-medium">
                            {toParty.label}
                          </span>
                        ) : (
                          <span className="font-mono text-muted-foreground truncate block max-w-[140px]">
                            {tx.to || tx.from || "System"}
                          </span>
                        )}
                      </div>
                    )}
                  </td>

                  {/* Package or Feature */}
                  <td className="px-5 py-3.5">
                    <div className="min-w-0">
                      <p className="text-xs font-medium text-foreground truncate max-w-[200px]">
                        {tx.package_or_feature || tx.description || label}
                      </p>
                      {tx.payos_order_code != null && (
                        <p className="font-mono text-[11px] text-muted-foreground">
                          Order #{tx.payos_order_code}
                        </p>
                      )}
                    </div>
                  </td>

                  {/* Paid VND */}
                  <td className="px-5 py-3.5 text-right tabular-nums text-xs font-semibold text-foreground">
                    {tx.paid_vnd != null ? formatVND(tx.paid_vnd) : "—"}
                  </td>

                  {/* Coins */}
                  <td className="px-5 py-3.5 text-right tabular-nums text-xs font-semibold">
                    {tx.coins != null ? (
                      <span
                        className={
                          isPositive
                            ? "text-emerald-600 dark:text-emerald-400"
                            : "text-muted-foreground"
                        }
                      >
                        {isPositive ? "+" : "-"}
                        {Math.abs(tx.coins).toLocaleString("en-US")}
                      </span>
                    ) : (
                      <span className="text-muted-foreground font-normal">
                        —
                      </span>
                    )}
                  </td>

                  {/* Status */}
                  <td className="px-5 py-3.5">
                    <div className="flex flex-col items-start gap-1">
                      <TxStatusBadge
                        status={tx.status}
                        isStalePending={isStale}
                      />
                      {tx.failure_reason && (
                        <p
                          className="text-[10px] text-destructive truncate max-w-[160px]"
                          title={tx.failure_reason}
                        >
                          {tx.failure_reason}
                        </p>
                      )}
                    </div>
                  </td>

                  {/* Date (UTC+7) */}
                  <td className="px-5 py-3.5 text-right text-xs text-muted-foreground tabular-nums whitespace-nowrap">
                    {formatDateTime(tx.created_at)}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {selectedTx && (
        <TransactionDetailDrawer
          tx={selectedTx}
          onClose={() => onDrawerClose?.()}
          onUpdated={onTxUpdated}
        />
      )}
    </>
  );
}
