"use client";

import { useState } from "react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { ArrowUpRight, Check, Search, Store } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { EmptyState } from "@/components/ui/empty-state";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { DataTablePagination } from "@/components/data-table/data-table-pagination";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { useFormatters } from "@/lib/i18n/use-formatters";
import {
  getCatalog,
  getCatalogItem,
  marketplaceKeys,
  marketplacePageSize,
} from "../api/marketplace";
import {
  catalogSortSchema,
  type CatalogItem,
} from "../schemas/marketplace-schema";
import {
  CoinPrice,
  ListingCopy,
  ListingPreview,
  MarketplaceBack,
  MarketplaceError,
  MarketplaceLoading,
  MarketplaceShell,
  marketplaceRoutes,
} from "./marketplace-shared";
import { CheckoutDialog } from "./checkout-dialog";

export function useMarketplaceParams() {
  const params = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  const rawOffset = Number(params.get("offset"));
  const offset =
    Number.isSafeInteger(rawOffset) && rawOffset >= 0 && rawOffset <= 10_000
      ? rawOffset
      : 0;
  function update(values: Record<string, string>) {
    const next = new URLSearchParams(params);
    for (const [key, value] of Object.entries(values)) {
      if (value && value !== "0") next.set(key, value);
      else next.delete(key);
    }
    router.push(`${pathname}${next.size ? `?${next}` : ""}`, { scroll: false });
  }
  return { params, offset, update };
}
export function CatalogScreen() {
  const t = useTranslations("marketplace");
  const user = useSessionUser();
  const { params, offset, update } = useMarketplaceParams();
  const q = Array.from(params.get("q")?.trim() ?? "")
    .slice(0, 100)
    .join("");
  const parsedSort = catalogSortSchema.safeParse(params.get("sort"));
  const sort = parsedSort.success ? parsedSort.data : "newest";
  const catalog = useQuery({
    queryKey: [...marketplaceKeys.catalog(user.id), { q, sort, offset }],
    queryFn: ({ signal }) => getCatalog({ q, sort, offset }, signal),
    retry: false,
  });
  return (
    <MarketplaceShell>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <form
          key={q}
          className="flex w-full gap-2 sm:max-w-md"
          onSubmit={(event) => {
            event.preventDefault();
            const data = new FormData(event.currentTarget);
            update({ q: String(data.get("q") ?? "").trim(), offset: "0" });
          }}
        >
          <div className="relative min-w-0 flex-1">
            <Search
              aria-hidden
              className="pointer-events-none absolute left-3 top-3.5 size-4 text-muted-foreground"
            />
            <Input
              aria-label={t("search")}
              name="q"
              defaultValue={q}
              maxLength={100}
              placeholder={t("searchPlaceholder")}
              className="h-11 pl-9"
            />
          </div>
          <Button type="submit" variant="outline" className="h-11">
            {t("searchSubmit")}
          </Button>
        </form>
        <Select
          value={sort}
          onValueChange={(value) => update({ sort: value, offset: "0" })}
        >
          <SelectTrigger aria-label={t("sort")} className="h-11 w-full sm:w-52">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {catalogSortSchema.options.map((value) => (
              <SelectItem key={value} value={value}>
                {t(value)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      {catalog.isPending ? (
        <MarketplaceLoading />
      ) : catalog.isError ? (
        <MarketplaceError
          error={catalog.error}
          retry={() => void catalog.refetch()}
          fetching={catalog.isFetching}
        />
      ) : (
        <>
          {!catalog.data.items.length ? (
            <EmptyState
              icons={[Store]}
              title={q ? t("searchEmpty") : t("emptyTitle")}
              description={q ? undefined : t("emptyDescription")}
              isSearch={!!q}
              searchQuery={q}
              action={
                q
                  ? {
                      label: t("clearSearch"),
                      onClick: () => update({ q: "", offset: "0" }),
                    }
                  : { label: t("sell"), href: marketplaceRoutes.new }
              }
            />
          ) : (
            <div className="grid gap-x-6 gap-y-8 sm:grid-cols-2 xl:grid-cols-3">
              {catalog.data.items.map((item) => (
                <CatalogCard key={item.id} item={item} />
              ))}
            </div>
          )}
          <DataTablePagination
            offset={offset}
            pageSize={marketplacePageSize}
            total={Math.min(
              catalog.data.pagination.total,
              10_000 + marketplacePageSize,
            )}
            disabled={catalog.isFetching}
            onPageChange={(page) =>
              update({ offset: String((page - 1) * marketplacePageSize) })
            }
            label={t("catalog")}
          />
        </>
      )}
    </MarketplaceShell>
  );
}
function CatalogCard({ item }: { item: CatalogItem }) {
  const t = useTranslations("marketplace");
  return (
    <article className="group min-w-0">
      <Link
        href={marketplaceRoutes.detail(item.id)}
        className="block rounded-xl outline-offset-4 focus-visible:outline-2 focus-visible:outline-ring"
      >
        <div className="relative">
          <ListingPreview
            path={item.preview_url}
            title={item.title}
            className="transition-colors duration-(--duration-fast) group-hover:bg-muted motion-reduce:transition-none"
          />
          {(item.already_owned || item.is_own_listing) && (
            <Badge className="absolute left-3 top-3 gap-1.5 bg-background text-foreground shadow-xs">
              <Check aria-hidden className="size-3.5" />
              {item.is_own_listing ? t("ownListing") : t("owned")}
            </Badge>
          )}
        </div>
        <div className="mt-4 flex items-start justify-between gap-3">
          <h2 className="min-w-0 break-words text-lg font-semibold tracking-tight">
            {item.title}
          </h2>
          <ArrowUpRight
            aria-hidden
            className="mt-1 size-4 shrink-0 text-muted-foreground transition-transform duration-(--duration-fast) group-hover:-translate-y-0.5 group-hover:translate-x-0.5 motion-reduce:transform-none motion-reduce:transition-none"
          />
        </div>
        <p className="mt-1 truncate text-xs text-muted-foreground">
          {t("by", { name: item.seller.display_name })}
        </p>
        <p className="mt-3 line-clamp-2 min-h-10 break-words text-sm leading-5 text-muted-foreground">
          {item.description || item.default_personality}
        </p>
        <CoinPrice amount={item.price_coins} className="mt-4 text-sm" />
      </Link>
    </article>
  );
}
export function CatalogDetailScreen({ id }: { id: string }) {
  const user = useSessionUser();
  const listing = useQuery({
    queryKey: marketplaceKeys.detail(user.id, id),
    queryFn: ({ signal }) => getCatalogItem(id, signal),
    retry: false,
  });
  return (
    <div className="mx-auto max-w-5xl space-y-6 pb-10">
      <MarketplaceBack />
      {listing.isPending ? (
        <MarketplaceLoading />
      ) : listing.isError ? (
        <MarketplaceError
          error={listing.error}
          retry={() => void listing.refetch()}
          fetching={listing.isFetching}
        />
      ) : (
        <CatalogDetail
          key={listing.data.id}
          item={listing.data}
          onReload={() => void listing.refetch()}
        />
      )}
    </div>
  );
}
function CatalogDetail({
  item,
  onReload,
}: {
  item: CatalogItem;
  onReload: () => void;
}) {
  const t = useTranslations("marketplace");
  const fmt = useFormatters();
  const [buying, setBuying] = useState(false);
  return (
    <>
      <div className="grid items-start gap-8 md:grid-cols-2 lg:gap-12">
        <div className="space-y-3">
          <ListingPreview path={item.preview_url} title={item.title} />
          <p className="text-xs leading-relaxed text-muted-foreground">
            {t("previewHint")}
          </p>
        </div>
        <div className="min-w-0 space-y-7">
          <header className="space-y-3">
            <p className="text-sm text-muted-foreground">
              {t("by", { name: item.seller.display_name })}
            </p>
            <h1 className="break-words text-3xl font-semibold tracking-tight sm:text-4xl">
              {item.title}
            </h1>
            <CoinPrice amount={item.price_coins} className="text-xl" />
          </header>
          <ListingCopy
            description={item.description}
            personality={item.default_personality}
          />
          <div className="space-y-3 border-t pt-6">
            {item.is_own_listing ? (
              <Button asChild variant="outline" className="min-h-11 w-full">
                <Link href={marketplaceRoutes.listing(item.id)}>
                  {t("ownListing")}
                </Link>
              </Button>
            ) : item.already_owned ? (
              <>
                <p className="flex items-center gap-2 text-sm font-medium">
                  <Check aria-hidden className="size-4" />
                  {t("owned")}
                </p>
                <Button asChild className="min-h-11 w-full">
                  <Link href="/avatar-studio">{t("inventory")}</Link>
                </Button>
              </>
            ) : (
              <Button
                onClick={() => setBuying(true)}
                className="min-h-12 w-full"
              >
                {t("buy", { price: fmt.coins(item.price_coins) })}
              </Button>
            )}
            <p className="text-xs leading-relaxed text-muted-foreground">
              {t("coinsOnly")}
            </p>
          </div>
        </div>
      </div>
      {buying && (
        <CheckoutDialog
          item={item}
          onClose={() => setBuying(false)}
          onReload={onReload}
        />
      )}
    </>
  );
}
