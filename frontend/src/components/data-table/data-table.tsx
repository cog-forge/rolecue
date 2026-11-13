import type { ReactNode } from "react";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { TableSkeleton } from "@/components/ui/skeleton-presets";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { cn } from "@/lib/utils";

export type DataTableColumn<T> = {
  id: string;
  header: string;
  cell: (row: T) => ReactNode;
  className?: string;
};

export interface DataTableProps<T> {
  rows: T[];
  columns: DataTableColumn<T>[];
  getRowId: (row: T) => string;
  label: string;
  isLoading?: boolean;
  error?: Error | unknown;
  onRetry?: () => void;
  emptyState?: ReactNode;
  className?: string;
}

export function DataTable<T>({
  rows,
  columns,
  getRowId,
  label,
  isLoading = false,
  error,
  onRetry,
  emptyState,
  className,
}: DataTableProps<T>) {
  return (
    <div
      className={cn(
        "min-w-0 overflow-hidden rounded-xl border bg-card/60 shadow-xs backdrop-blur-xs",
        !isLoading &&
          !error &&
          rows.length === 0 &&
          "border-2 border-dashed bg-transparent shadow-none",
        className,
      )}
    >
      {isLoading ? (
        <TableSkeleton columns={columns.length} label={label} />
      ) : error ? (
        <ErrorState variant="inline" error={error} onRetry={onRetry} />
      ) : rows.length === 0 ? (
        (emptyState ?? <EmptyState variant="plain" />)
      ) : (
        <Table aria-label={label} className="min-w-3xl text-left">
          <TableHeader className="bg-muted/40">
            <TableRow className="hover:bg-transparent">
              {columns.map((column) => (
                <TableHead
                  key={column.id}
                  scope="col"
                  className="h-12 px-5 text-xs font-semibold text-muted-foreground uppercase tracking-wider"
                >
                  {column.header}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <TableRow
                key={getRowId(row)}
                className="align-top transition-colors hover:bg-muted/20"
              >
                {columns.map((column) => (
                  <TableCell
                    key={column.id}
                    className={column.className ?? "px-5 py-4"}
                  >
                    {column.cell(row)}
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  );
}
