"use client";

import { useState } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import {
  ArrowDown,
  ArrowUp,
  Check,
  ChevronLeft,
  ChevronRight,
  Copy,
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
import { cn } from "@/lib/utils";
import { describeTransaction } from "@/features/billing/lib/describe-transaction";
import { adminBillingKeys, getBillingTransactionByID } from "../api/billing";
import {
  formatCoins,
  formatDateTime,
  formatPlural,
  formatVND,
} from "../lib/format";
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

// Status is the only coloured mark in a row: a pale tint with dark text of the same hue.
const statusStyles: Record<string, string> = {
  success:
    "bg-emerald-50 text-emerald-900 dark:bg-emerald-950 dark:text-emerald-200",
  pending: "bg-amber-50 text-amber-900 dark:bg-amber-950 dark:text-amber-200",
  failed: "bg-rose-50 text-rose-900 dark:bg-rose-950 dark:text-rose-200",
  cancelled: "bg-muted text-muted-foreground",
};

const statusLabels: Record<string, string> = {
  success: "Paid",
  pending: "Waiting",
  failed: "Failed",
  cancelled: "Cancelled",
};

const typeLabels: Record<string, string> = {
  topup: "PayOS top-up",
  spend: "Coin spend",
  refund: "Coin refund",
};

function StatusPill({ tx }: { tx: AdminTransactionItem }) {
  // Coin rows are always settled; only PayOS payments can wait, fail or be cancelled.
  const label = tx.type === "topup" ? statusLabels[tx.status] : "Settled";
  return (
    <Badge
      variant="outline"
      className={cn(
        "flex-none border-transparent text-xs font-medium whitespace-nowrap",
        statusStyles[tx.status] ?? statusStyles.cancelled,
      )}
    >
      {label ?? tx.status}
    </Badge>
  );
}

function rowTitle(tx: AdminTransactionItem) {
  const described = describeTransaction(tx.description);
  if (described.kind === "other" && tx.package_or_feature) {
    return { ...described, title: tx.package_or_feature };
  }
  return described;
}

function Amount({ tx }: { tx: AdminTransactionItem }) {
  if (tx.type === "topup") {
    const paid = tx.status === "success";
    return (
      <>
        <span
          className={cn(
            "block font-medium",
            paid ? "text-foreground" : "text-muted-foreground line-through",
          )}
        >
          {formatVND(tx.paid_vnd)}
        </span>
        <span className="block text-xs text-muted-foreground">
          {paid ? `+${formatCoins(tx.coin_amount)}` : "No coins added"}
        </span>
      </>
    );
  }
  return (
    <span className="block font-medium text-foreground">
      {tx.coin_amount > 0 ? "+" : ""}
      {formatCoins(tx.coin_amount)}
    </span>
  );
}

function DetailRow({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex items-start justify-between gap-4">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="min-w-0 text-right break-words text-foreground">
        {children}
      </dd>
    </div>
  );
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
  const [copied, setCopied] = useState(false);
  const [selectedTx, setSelectedTx] = useState<AdminTransactionItem | null>(
    null,
  );

  // Fetch the authoritative detail via GET /admin/billing/transactions/:id when the drawer opens.
  const detailQuery = useQuery({
    queryKey: adminBillingKeys.transactionDetail(selectedTx?.id ?? ""),
    queryFn: ({ signal }) => getBillingTransactionByID(selectedTx!.id, signal),
    enabled: Boolean(selectedTx?.id),
    staleTime: 30_000,
  });

  const activeDetail = detailQuery.data ?? selectedTx;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  const copyId = (id: string) => {
    void navigator.clipboard.writeText(id);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  // A sort glyph on the active column only.
  const sortHeader = (col: string, label: string, align?: "right") => (
    <button
      type="button"
      onClick={() => onSortChange(col)}
      className={cn(
        "inline-flex items-center gap-1 font-normal text-muted-foreground hover:text-foreground",
        align === "right" && "ml-auto",
      )}
    >
      {label}
      {sortBy === col &&
        (sortOrder === "asc" ? (
          <ArrowUp className="size-3.5" aria-hidden />
        ) : (
          <ArrowDown className="size-3.5" aria-hidden />
        ))}
    </button>
  );

  if (isLoading) {
    return (
      <div className="rounded-xl bg-card p-4 ring-1 ring-foreground/10">
        <div className="space-y-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full rounded-lg" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="rounded-xl bg-card ring-1 ring-foreground/10">
      <div className="overflow-x-auto">
        <Table className="min-w-[760px]">
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <TableHead className="w-[150px] text-xs">
                {sortHeader("created_at", "Date")}
              </TableHead>
              <TableHead className="w-[220px] text-xs font-normal text-muted-foreground">
                User
              </TableHead>
              <TableHead className="text-xs font-normal text-muted-foreground">
                Transaction
              </TableHead>
              <TableHead className="w-[150px] text-right text-xs">
                <span className="flex justify-end">
                  {sortHeader("amount", "Amount", "right")}
                </span>
              </TableHead>
              <TableHead className="w-[110px] text-xs">
                {sortHeader("status", "Status")}
              </TableHead>
            </TableRow>
          </TableHeader>

          <TableBody>
            {items.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={5}
                  className="h-32 text-center text-sm text-muted-foreground"
                >
                  No transactions match the current filters.
                </TableCell>
              </TableRow>
            ) : (
              items.map((tx) => {
                const title = rowTitle(tx);
                return (
                  <TableRow
                    key={tx.id}
                    onClick={() => setSelectedTx(tx)}
                    className="cursor-pointer"
                  >
                    <TableCell className="align-top text-sm whitespace-nowrap text-foreground tabular-nums">
                      <time dateTime={tx.created_at || undefined}>
                        {formatDateTime(tx.created_at)}
                      </time>
                    </TableCell>

                    <TableCell className="max-w-[220px] align-top">
                      {tx.user_name || tx.user_email ? (
                        <>
                          <Link
                            href={`/admin/accounts?search=${encodeURIComponent(tx.user_email || tx.user_name)}`}
                            onClick={(e) => e.stopPropagation()}
                            className="block truncate text-sm text-foreground hover:underline"
                          >
                            {tx.user_name || "Unnamed user"}
                          </Link>
                          {tx.user_email && (
                            <span className="block truncate text-xs text-muted-foreground">
                              {tx.user_email}
                            </span>
                          )}
                        </>
                      ) : (
                        <span className="text-sm text-muted-foreground">
                          {tx.user_id ? "Unknown user" : "System"}
                        </span>
                      )}
                    </TableCell>

                    <TableCell className="align-top whitespace-normal">
                      <button
                        type="button"
                        onClick={(e) => {
                          e.stopPropagation();
                          setSelectedTx(tx);
                        }}
                        aria-label={`View details of ${title.title} on ${formatDateTime(tx.created_at)}`}
                        className="block text-left text-sm text-foreground hover:underline"
                      >
                        {title.title}
                      </button>
                      <span className="block text-xs text-muted-foreground">
                        {[
                          typeLabels[tx.type] ?? tx.type,
                          title.kind !== "topup" && title.detail,
                          tx.payos_order_code != null &&
                            `Order #${tx.payos_order_code}`,
                        ]
                          .filter(Boolean)
                          .join(" · ")}
                      </span>
                    </TableCell>

                    <TableCell className="text-right align-top text-sm tabular-nums">
                      <Amount tx={tx} />
                    </TableCell>

                    <TableCell className="align-top">
                      <StatusPill tx={tx} />
                    </TableCell>
                  </TableRow>
                );
              })
            )}
          </TableBody>
        </Table>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border/60 px-4 py-3">
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <span>
            {items.length > 0
              ? `${(page - 1) * pageSize + 1}–${(page - 1) * pageSize + items.length} of `
              : "0 of "}
            {formatPlural(total, "transaction", "transactions")}
          </span>
          <span aria-hidden>·</span>
          <span>Rows</span>
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
            <ChevronLeft className="size-3.5" aria-hidden />
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
            <ChevronRight className="size-3.5" aria-hidden />
          </Button>
        </div>
      </div>

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
              Read only. Admins cannot change transactions or wallet balances.
            </SheetDescription>
          </SheetHeader>

          {activeDetail && (
            <div className="space-y-6 px-4 pb-6 text-sm">
              <div className="space-y-1">
                <p className="text-base font-semibold text-foreground">
                  {rowTitle(activeDetail).title}
                </p>
                <div className="flex items-center gap-2">
                  <StatusPill tx={activeDetail} />
                  <span className="text-xs text-muted-foreground">
                    {typeLabels[activeDetail.type] ?? activeDetail.type}
                  </span>
                </div>
              </div>

              <dl className="space-y-2.5">
                <DetailRow label="Created at">
                  <time
                    dateTime={activeDetail.created_at || undefined}
                    className="tabular-nums"
                  >
                    {formatDateTime(activeDetail.created_at)}
                  </time>
                </DetailRow>
                <DetailRow label="Paid">
                  {activeDetail.type === "topup"
                    ? formatVND(activeDetail.paid_vnd)
                    : "—"}
                </DetailRow>
                <DetailRow label="Coins">
                  {activeDetail.coin_amount > 0 ? "+" : ""}
                  {formatCoins(activeDetail.coin_amount)}
                </DetailRow>
                <DetailRow label="User">
                  {activeDetail.user_name || "—"}
                  {activeDetail.user_email && (
                    <span className="block text-xs text-muted-foreground">
                      {activeDetail.user_email}
                    </span>
                  )}
                </DetailRow>
              </dl>

              <dl className="space-y-2.5 border-t border-border/60 pt-4">
                <DetailRow label="Transaction ID">
                  <span className="inline-flex items-center gap-1.5">
                    <code className="font-mono text-xs">{activeDetail.id}</code>
                    <button
                      type="button"
                      onClick={() => copyId(activeDetail.id)}
                      aria-label="Copy transaction ID"
                      className="rounded p-1 text-muted-foreground hover:bg-muted hover:text-foreground"
                    >
                      {copied ? (
                        <Check className="size-3.5" aria-hidden />
                      ) : (
                        <Copy className="size-3.5" aria-hidden />
                      )}
                    </button>
                  </span>
                </DetailRow>
                <DetailRow label="PayOS order">
                  {activeDetail.payos_order_code != null
                    ? `#${activeDetail.payos_order_code}`
                    : "—"}
                </DetailRow>
                <DetailRow label="From">
                  <code className="font-mono text-xs">
                    {activeDetail.from || "—"}
                  </code>
                </DetailRow>
                <DetailRow label="To">
                  <code className="font-mono text-xs">
                    {activeDetail.to || "—"}
                  </code>
                </DetailRow>
                <DetailRow label="Ledger text">
                  <span className="text-xs">
                    {activeDetail.description || "—"}
                  </span>
                </DetailRow>
              </dl>
            </div>
          )}
        </SheetContent>
      </Sheet>
    </div>
  );
}
