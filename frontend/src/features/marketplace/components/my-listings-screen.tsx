"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { ArrowUpRight, Store } from "lucide-react";
import { DataTablePagination } from "@/components/data-table/data-table-pagination";
import { EmptyState } from "@/components/ui/empty-state";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { useFormatters } from "@/lib/i18n/use-formatters";
import {
  getMyListings,
  marketplaceKeys,
  marketplacePageSize,
} from "../api/marketplace";
import {
  CoinPrice,
  ListingPreview,
  ListingStatusBadge,
  MarketplaceError,
  MarketplaceLoading,
  MarketplaceShell,
  marketplaceRoutes,
} from "./marketplace-shared";
import { useMarketplaceParams } from "./catalog-screen";

export function MyListingsScreen() {
  const t = useTranslations("marketplace");
  const fmt = useFormatters();
  const user = useSessionUser();
  const { offset, update } = useMarketplaceParams();
  const listings = useQuery({
    queryKey: [...marketplaceKeys.own(user.id), { offset }],
    queryFn: ({ signal }) => getMyListings(offset, signal),
    retry: false,
  });
  return (
    <MarketplaceShell own>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-muted-foreground">
          {t("myEmptyDescription")}
        </p>
        <Link
          href={user.role === "recruiter" ? "/recruiter/billing" : "/billing"}
          className="inline-flex min-h-11 items-center gap-1.5 text-sm font-medium underline-offset-4 hover:underline"
        >
          {t("history")}
          <ArrowUpRight aria-hidden className="size-4" />
        </Link>
      </div>
      {listings.isPending ? (
        <MarketplaceLoading />
      ) : listings.isError ? (
        <MarketplaceError
          error={listings.error}
          retry={() => void listings.refetch()}
          fetching={listings.isFetching}
        />
      ) : (
        <>
          {!listings.data.items.length ? (
            <EmptyState
              icons={[Store]}
              title={t("myEmptyTitle")}
              description={t("myEmptyDescription")}
              action={{ label: t("sell"), href: marketplaceRoutes.new }}
            />
          ) : (
            <div className="divide-y border-y">
              {listings.data.items.map((item) => (
                <Link
                  key={item.id}
                  href={marketplaceRoutes.listing(item.id)}
                  className="group flex min-w-0 items-start gap-4 py-5 outline-offset-4 sm:items-center sm:gap-6"
                >
                  <ListingPreview
                    path={item.preview_url}
                    title={item.title}
                    className="size-20 shrink-0 sm:size-24 [&_span]:hidden [&_svg]:size-6"
                  />
                  <div className="min-w-0 flex-1 space-y-2">
                    <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
                      <h2 className="break-words font-semibold">
                        {item.title}
                      </h2>
                      <ListingStatusBadge status={item.status} />
                    </div>
                    <p className="line-clamp-2 break-words text-sm text-muted-foreground">
                      {item.review_note ||
                        item.description ||
                        item.default_personality}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      {t("updatedAt")} ·{" "}
                      {fmt.date(item.updated_at, "localDateTime")}
                    </p>
                    <CoinPrice
                      amount={item.price_coins}
                      className="text-sm sm:hidden"
                    />
                  </div>
                  <CoinPrice
                    amount={item.price_coins}
                    className="hidden text-sm sm:inline-flex"
                  />
                  <ArrowUpRight
                    aria-hidden
                    className="mt-1 size-4 shrink-0 text-muted-foreground transition-transform duration-(--duration-fast) group-hover:-translate-y-0.5 group-hover:translate-x-0.5 motion-reduce:transform-none motion-reduce:transition-none"
                  />
                </Link>
              ))}
            </div>
          )}
          <DataTablePagination
            offset={offset}
            pageSize={marketplacePageSize}
            total={Math.min(
              listings.data.pagination.total,
              10_000 + marketplacePageSize,
            )}
            disabled={listings.isFetching}
            onPageChange={(page) =>
              update({ offset: String((page - 1) * marketplacePageSize) })
            }
            label={t("myListings")}
          />
        </>
      )}
    </MarketplaceShell>
  );
}
