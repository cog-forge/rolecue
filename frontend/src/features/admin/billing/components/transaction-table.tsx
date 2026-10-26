"use client";

import { useState } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import {
  ArrowDown,
  ArrowUp,
  ArrowUpDown,
  Check,
  ChevronLeft,
  ChevronRight,
  Copy,
  Eye,
  ExternalLink,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { adminBillingKeys, getBillingTransactionByID } from "../api/billing";
import { formatVND, formatCoins, formatPlural } from "../lib/format";
import type { AdminTransactionItem } from "../schemas/billing-schema";

type TransactionTableProps = {
  items: AdminTransactionItem[];
  total: number;
  page: number;
  pageSize: number;
  isLoading: boolean;
  sortBy: string;
  sortOrder: "asc" | "desc";
  onSortChange: (col: string) => void;
  onPageChange: (page: number) => void;
  onPageSizeChange: (size: number) => void;
};

function statusBadgeVariant(status: string): string {
  switch (status) {
    case "success":
      return "border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300";
    case "pending":
      return "border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300";
    case "failed":
      return "border-rose-500/30 bg-rose-500/10 text-rose-700 dark:text-rose-300";
    case "cancelled":
    default:
      return "border-slate-500/30 bg-slate-500/10 text-slate-700 dark:text-slate-300";
  }
}

function typeLabel(txType: string): { label: string; badgeClass: string } {
  switch (txType) {
    case "topup":
      return {
        label: "PayOS Top-up",
        badgeClass:
          "border-blue-500/30 bg-blue-500/10 text-blue-700 dark:text-blue-300",
      };
    case "refund":
      return {
        label: "Slot Refund",
        badgeClass:
          "border-violet-500/30 bg-violet-500/10 text-violet-700 dark:text-violet-300",
      };
    case "spend":
    default:
      return {
        label: "Coin Spend",
        badgeClass:
          "border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300",
      };
  }
}

export function TransactionTable({
  items,
  total,
  page,
  pageSize,
  isLoading,
  sortBy,
  sortOrder,
  onSortChange,
  onPageChange,
  onPageSizeChange,
}: TransactionTableProps) {
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [selectedTx, setSelectedTx] = useState<AdminTransactionItem | null>(
    null,
  );

  // Fetch authoritative transaction detail via GET /admin/billing/transactions/:id when drawer opens
  const detailQuery = useQuery({
    queryKey: adminBillingKeys.transactionDetail(selectedTx?.id ?? ""),
    queryFn: ({ signal }) => getBillingTransactionByID(selectedTx!.id, signal),
    enabled: Boolean(selectedTx?.id),
    staleTime: 30_000,
  });

  const activeDetail = detailQuery.data ?? selectedTx;

  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  const handleCopy = (text: string, e: React.MouseEvent) => {
    e.stopPropagation();
    void navigator.clipboard.writeText(text);
    setCopiedId(text);
    setTimeout(() => setCopiedId(null), 1500);
  };

  const renderSortIcon = (col: string) => {
    if (sortBy !== col) {
      return <ArrowUpDown className="ml-1 inline size-3.5 opacity-40" />;
    }
    return sortOrder === "asc" ? (
      <ArrowUp className="ml-1 inline size-3.5 text-foreground" />
    ) : (
      <ArrowDown className="ml-1 inline size-3.5 text-foreground" />
    );
  };

  if (isLoading) {
    return (
      <div className="rounded-xl border border-border/80 bg-card p-4 shadow-xs">
        <div className="space-y-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full rounded-lg" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="rounded-xl border border-border/80 bg-card shadow-xs">
      <div className="overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <TableHead className="w-[180px] text-xs">
                <button
                  type="button"
                  onClick={() => onSortChange("id")}
                  className="inline-flex items-center font-semibold"
                >
                  Transaction / Order {renderSortIcon("id")}
                </button>
              </TableHead>
              <TableHead className="text-xs font-semibold">User</TableHead>
              <TableHead className="text-xs font-semibold">
                Type & Package / Feature
              </TableHead>
              <TableHead className="text-right text-xs">
                <button
                  type="button"
                  onClick={() => onSortChange("amount")}
                  className="inline-flex items-center font-semibold"
                >
                  Paid (VND) {renderSortIcon("amount")}
                </button>
              </TableHead>
              <TableHead className="text-right text-xs">
                <button
                  type="button"
                  onClick={() => onSortChange("coins")}
                  className="inline-flex items-center font-semibold"
                >
                  Coins {renderSortIcon("coins")}
                </button>
              </TableHead>
              <TableHead className="text-xs">
                <button
                  type="button"
                  onClick={() => onSortChange("status")}
                  className="inline-flex items-center font-semibold"
                >
                  Status {renderSortIcon("status")}
                </button>
              </TableHead>
              <TableHead className="text-right text-xs font-semibold">
                Inspect
              </TableHead>
            </TableRow>
          </TableHeader>

          <TableBody>
            {items.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={7}
                  className="h-40 text-center text-sm text-muted-foreground"
                >
                  No transactions match the current filters.
                </TableCell>
              </TableRow>
            ) : (
              items.map((tx) => {
                const typeMeta = typeLabel(tx.type);
                const isTopup = tx.type === "topup";
                const coinSign = tx.coin_amount > 0 ? "+" : "";

                return (
                  <TableRow
                    key={tx.id}
                    onClick={() => setSelectedTx(tx)}
                    className="cursor-pointer transition-colors hover:bg-muted/40"
                  >
                    {/* Transaction ID & PayOS Order Code */}
                    <TableCell className="align-middle">
                      <div className="flex items-center gap-1.5">
                        <code className="font-mono text-xs text-foreground">
                          {tx.id.slice(0, 8)}…
                        </code>
                        <button
                          type="button"
                          onClick={(e) => handleCopy(tx.id, e)}
                          title="Copy full Transaction UUID"
                          aria-label={`Copy transaction ID ${tx.id}`}
                          className="rounded p-1 text-muted-foreground hover:bg-muted hover:text-foreground"
                        >
                          {copiedId === tx.id ? (
                            <Check className="size-3 text-emerald-600" />
                          ) : (
                            <Copy className="size-3" />
                          )}
                        </button>
                      </div>
                      {tx.payos_order_code != null && (
                        <span className="mt-0.5 block font-mono text-[11px] text-muted-foreground">
                          Order #{tx.payos_order_code}
                        </span>
                      )}
                    </TableCell>

                    {/* User Name & Email linking to Accounts page */}
                    <TableCell className="align-middle">
                      {tx.user_name || tx.user_email ? (
                        <div>
                          <Link
                            href={`/admin/accounts?search=${encodeURIComponent(tx.user_email || tx.user_name)}`}
                            onClick={(e) => e.stopPropagation()}
                            className="inline-flex items-center gap-1 text-xs font-medium text-foreground hover:underline"
                          >
                            {tx.user_name || "Unnamed User"}
                            <ExternalLink className="size-3 text-muted-foreground" />
                          </Link>
                          {tx.user_email && (
                            <span className="block text-[11px] text-muted-foreground">
                              {tx.user_email}
                            </span>
                          )}
                        </div>
                      ) : tx.user_id ? (
                        <code className="font-mono text-xs text-muted-foreground">
                          {tx.user_id.slice(0, 12)}…
                        </code>
                      ) : (
                        <span className="text-xs text-muted-foreground">
                          System
                        </span>
                      )}
                    </TableCell>

                    {/* Type & Package / Feature */}
                    <TableCell className="align-middle">
                      <div className="flex flex-col gap-1">
                        <div className="flex items-center gap-1.5">
                          <Badge
                            variant="outline"
                            className={`text-[10px] font-medium ${typeMeta.badgeClass}`}
                          >
                            {typeMeta.label}
                          </Badge>
                        </div>
                        <span className="text-xs font-medium text-foreground">
                          {tx.package_or_feature || tx.description || "—"}
                        </span>
                      </div>
                    </TableCell>

                    {/* Paid (VND) — separate from Coins */}
                    <TableCell className="text-right align-middle font-mono text-xs font-semibold tabular-nums">
                      {isTopup && tx.paid_vnd > 0 ? (
                        <span className="text-foreground">
                          {formatVND(tx.paid_vnd)}
                        </span>
                      ) : (
                        <span className="text-muted-foreground">—</span>
                      )}
                    </TableCell>

                    {/* Coins — never labeled as VND */}
                    <TableCell className="text-right align-middle font-mono text-xs font-semibold tabular-nums">
                      <span
                        className={
                          tx.coin_amount > 0
                            ? "text-emerald-600 dark:text-emerald-400"
                            : tx.coin_amount < 0
                              ? "text-amber-600 dark:text-amber-400"
                              : "text-muted-foreground"
                        }
                      >
                        {coinSign}
                        {formatCoins(tx.coin_amount)}
                      </span>
                    </TableCell>

                    {/* Status Badge */}
                    <TableCell className="align-middle">
                      <Badge
                        variant="outline"
                        className={`text-[11px] font-medium capitalize ${statusBadgeVariant(tx.status)}`}
                      >
                        {tx.status}
                      </Badge>
                    </TableCell>

                    {/* Inspect Drawer Trigger */}
                    <TableCell className="text-right align-middle">
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={(e) => {
                          e.stopPropagation();
                          setSelectedTx(tx);
                        }}
                        aria-label={`Inspect transaction ${tx.id}`}
                        className="h-7 gap-1 px-2 text-xs"
                      >
                        <Eye className="size-3.5" />
                        Details
                      </Button>
                    </TableCell>
                  </TableRow>
                );
              })
            )}
          </TableBody>
        </Table>
      </div>

      {/* Pagination Footer */}
      <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border/60 px-4 py-3">
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <span>
            Showing{" "}
            <strong className="font-semibold text-foreground">
              {items.length}
            </strong>{" "}
            of{" "}
            <strong className="font-semibold text-foreground">
              {formatPlural(total, "transaction", "transactions")}
            </strong>
          </span>
          <span>·</span>
          <div className="flex items-center gap-1.5">
            <span>Rows per page:</span>
            <Select
              value={String(pageSize)}
              onValueChange={(v) => onPageSizeChange(Number(v))}
            >
              <SelectTrigger
                className="h-7 w-[72px] text-xs"
                aria-label="Rows per page"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="20">20</SelectItem>
                <SelectItem value="50">50</SelectItem>
                <SelectItem value="100">100</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground">
            Page {page} of {totalPages}
          </span>
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={page <= 1}
            onClick={() => onPageChange(page - 1)}
            className="h-7 px-2 text-xs"
          >
            <ChevronLeft className="size-3.5" />
            Previous
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={page >= totalPages}
            onClick={() => onPageChange(page + 1)}
            className="h-7 px-2 text-xs"
          >
            Next
            <ChevronRight className="size-3.5" />
          </Button>
        </div>
      </div>

      {/* Read-Only Transaction Detail Drawer */}
      <Sheet
        open={Boolean(selectedTx)}
        onOpenChange={(open) => {
          if (!open) setSelectedTx(null);
        }}
      >
        <SheetContent className="w-full overflow-y-auto sm:max-w-lg">
          <SheetHeader>
            <SheetTitle>Transaction Details</SheetTitle>
            <SheetDescription>
              Read-only ledger inspection. Administrator accounts cannot mutate
              transactions or wallet balances (KAN-191).
            </SheetDescription>
          </SheetHeader>

          {activeDetail && (
            <div className="mt-6 space-y-5 text-xs">
              <div className="space-y-2 rounded-lg border border-border/70 bg-muted/30 p-3.5">
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">Transaction ID</span>
                  <code className="font-mono text-[11px] font-medium">
                    {activeDetail.id}
                  </code>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">
                    PayOS Order Code
                  </span>
                  <span className="font-mono font-medium">
                    {activeDetail.payos_order_code != null
                      ? `#${activeDetail.payos_order_code}`
                      : "N/A (Internal Coin Ledger)"}
                  </span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">Status</span>
                  <Badge
                    variant="outline"
                    className={`capitalize ${statusBadgeVariant(activeDetail.status)}`}
                  >
                    {activeDetail.status}
                  </Badge>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">Type</span>
                  <span className="font-medium capitalize">
                    {typeLabel(activeDetail.type).label}
                  </span>
                </div>
              </div>

              <div className="space-y-2 rounded-lg border border-border/70 p-3.5">
                <h4 className="font-semibold text-foreground">
                  Financial Breakdown
                </h4>
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">
                    Package / Feature
                  </span>
                  <span className="font-medium">
                    {activeDetail.package_or_feature || "—"}
                  </span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">Paid (VND)</span>
                  <span className="font-mono font-semibold">
                    {activeDetail.type === "topup" && activeDetail.paid_vnd > 0
                      ? formatVND(activeDetail.paid_vnd)
                      : "0 ₫ (Internal Coin Transaction)"}
                  </span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">Coin Delta</span>
                  <span className="font-mono font-semibold">
                    {activeDetail.coin_amount > 0 ? "+" : ""}
                    {formatCoins(activeDetail.coin_amount)}
                  </span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">Raw Description</span>
                  <span className="text-right font-mono text-[11px]">
                    {activeDetail.description || "—"}
                  </span>
                </div>
              </div>

              <div className="space-y-2 rounded-lg border border-border/70 p-3.5">
                <h4 className="font-semibold text-foreground">
                  Account & Ledger Routing
                </h4>
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">User</span>
                  <span>
                    {activeDetail.user_name || "—"}{" "}
                    {activeDetail.user_email
                      ? `(${activeDetail.user_email})`
                      : ""}
                  </span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">From</span>
                  <code className="font-mono text-[11px]">
                    {activeDetail.from || "— (External Deposit)"}
                  </code>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">To</span>
                  <code className="font-mono text-[11px]">
                    {activeDetail.to || "— (System Charge)"}
                  </code>
                </div>
              </div>
            </div>
          )}
        </SheetContent>
      </Sheet>
    </div>
  );
}
