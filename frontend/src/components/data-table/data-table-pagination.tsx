"use client";

import { useId, useState } from "react";
import {
  ChevronsLeft,
  ChevronsRight,
  ChevronLeft,
  ChevronRight,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationItem,
} from "@/components/ui/pagination";

export function DataTablePagination({
  offset,
  pageSize,
  total,
  disabled,
  onPageChange,
  label,
}: {
  offset: number;
  pageSize: number;
  total: number;
  disabled: boolean;
  onPageChange: (page: number) => void;
  label: string;
}) {
  const id = useId();
  const [jump, setJump] = useState("");
  const page = Math.floor(offset / pageSize) + 1;
  const pages = Math.ceil(total / pageSize);
  const jumpPage = Number(jump);
  const validJump =
    !!jump && Number.isInteger(jumpPage) && jumpPage >= 1 && jumpPage <= pages;
  const numbers = [...new Set([1, pages, page - 1, page, page + 1])]
    .filter((n) => n >= 1 && n <= pages)
    .sort((a, b) => a - b);
  return (
    <div className="flex flex-wrap items-center justify-between gap-4 border-t pt-4">
      <p role="status" className="text-xs text-muted-foreground">
        {total
          ? `${offset + 1}–${Math.min(offset + pageSize, total)} of ${total} · Page ${page} of ${pages}`
          : "0 results"}
      </p>
      <div className="flex flex-wrap items-center gap-4">
        <Pagination aria-label={label} className="mx-0 w-auto">
          <PaginationContent className="flex-wrap gap-1">
            <PaginationItem>
              <Button
                variant="outline"
                className="size-11"
                aria-label="First page"
                disabled={disabled || page <= 1 || !pages}
                onClick={() => onPageChange(1)}
              >
                <ChevronsLeft aria-hidden />
              </Button>
            </PaginationItem>
            <PaginationItem>
              <Button
                variant="outline"
                className="size-11"
                aria-label="Previous page"
                disabled={disabled || page <= 1 || !pages}
                onClick={() => onPageChange(page - 1)}
              >
                <ChevronLeft aria-hidden />
              </Button>
            </PaginationItem>
            {numbers.map((number, index) => (
              <PaginationItem
                key={number}
                className={
                  number === page
                    ? "flex items-center gap-1"
                    : "hidden items-center gap-1 sm:flex"
                }
              >
                {index > 0 && number - numbers[index - 1] > 1 && (
                  <PaginationEllipsis className="hidden sm:flex" />
                )}
                <Button
                  variant={number === page ? "secondary" : "ghost"}
                  className="size-11"
                  aria-label={`Page ${number}`}
                  aria-current={number === page ? "page" : undefined}
                  disabled={disabled}
                  onClick={() => onPageChange(number)}
                >
                  {number}
                </Button>
              </PaginationItem>
            ))}
            <PaginationItem>
              <Button
                variant="outline"
                className="size-11"
                aria-label="Next page"
                disabled={disabled || page >= pages || !pages}
                onClick={() => onPageChange(page + 1)}
              >
                <ChevronRight aria-hidden />
              </Button>
            </PaginationItem>
            <PaginationItem>
              <Button
                variant="outline"
                className="size-11"
                aria-label="Last page"
                disabled={disabled || page >= pages || !pages}
                onClick={() => onPageChange(pages)}
              >
                <ChevronsRight aria-hidden />
              </Button>
            </PaginationItem>
          </PaginationContent>
        </Pagination>
        <form
          aria-label="Go to page"
          className="flex items-center gap-2"
          onSubmit={(event) => {
            event.preventDefault();
            if (!disabled && validJump) {
              onPageChange(jumpPage);
              setJump("");
            }
          }}
        >
          <label htmlFor={id} className="text-xs text-muted-foreground">
            Jump to page
          </label>
          <Input
            id={id}
            className="h-11 w-20"
            type="number"
            min={1}
            max={Math.max(1, pages)}
            step={1}
            inputMode="numeric"
            value={jump}
            onChange={(event) => setJump(event.target.value)}
            disabled={disabled || !pages}
          />
          <Button
            type="submit"
            variant="outline"
            className="h-11"
            disabled={disabled || !validJump}
          >
            Go
          </Button>
        </form>
      </div>
    </div>
  );
}
