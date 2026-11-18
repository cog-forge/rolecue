"use client";

import { useState } from "react";
import Link from "next/link";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { gooeyToast } from "goey-toast";
import { ArrowUpRight, ClipboardCheck } from "lucide-react";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
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
  adminAction,
  getAdminListing,
  getAdminListings,
  marketplaceKeys,
  marketplacePageSize,
} from "../api/marketplace";
import {
  listingStatusSchema,
  type AdminListing,
} from "../schemas/marketplace-schema";
import { useMarketplaceParams } from "./catalog-screen";
import {
  CoinPrice,
  ListingCopy,
  ListingPreview,
  ListingStatusBadge,
  MarketplaceBack,
  MarketplaceError,
  MarketplaceLoading,
  marketplaceRoutes,
} from "./marketplace-shared";
import { ListingActionDialog } from "./listing-action-dialog";

export function AdminMarketplaceScreen() {
  const t = useTranslations("marketplace");
  const fmt = useFormatters();
  const user = useSessionUser();
  const { params, offset, update } = useMarketplaceParams();
  const raw = params.get("status");
  const parsed = listingStatusSchema.safeParse(raw);
  const status =
    raw === "all" ? "all" : parsed.success ? parsed.data : "pending";
  const listings = useQuery({
    queryKey: [...marketplaceKeys.admin(user.id), { status, offset }],
    queryFn: ({ signal }) => getAdminListings(status, offset, signal),
    retry: false,
  });
  return (
    <div className="mx-auto max-w-6xl space-y-7 pb-10">
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div className="max-w-2xl space-y-2">
          <h1 className="text-3xl font-semibold tracking-tight">
            {t("reviewQueue")}
          </h1>
          <p className="text-sm leading-relaxed text-muted-foreground">
            {t("reviewSubtitle")}
          </p>
        </div>
        <Select
          value={status}
          onValueChange={(value) => update({ status: value, offset: "0" })}
        >
          <SelectTrigger
            aria-label={t("status")}
            className="min-h-11 w-full sm:w-48"
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t("all")}</SelectItem>
            {listingStatusSchema.options.map((value) => (
              <SelectItem key={value} value={value}>
                {t(`statusLabels.${value}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </header>
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
              icons={[ClipboardCheck]}
              title={t("queueEmpty")}
              description={t("queueEmptyDescription")}
            />
          ) : (
            <div className="divide-y border-y">
              {listings.data.items.map((item) => (
                <Link
                  key={item.id}
                  href={marketplaceRoutes.review(item.id)}
                  className="group flex min-w-0 items-start gap-4 py-5 sm:items-center sm:gap-6"
                >
                  <ListingPreview
                    path={item.preview_url}
                    title={item.title}
                    className="size-20 shrink-0 [&_span]:hidden [&_svg]:size-6"
                  />
                  <div className="min-w-0 flex-1 space-y-2">
                    <div className="flex flex-wrap items-center gap-3">
                      <h2 className="break-words font-semibold">
                        {item.title}
                      </h2>
                      <ListingStatusBadge status={item.status} />
                    </div>
                    <p className="break-words text-sm text-muted-foreground">
                      {t("by", { name: item.seller.display_name })}
                    </p>
                    <p className="text-xs text-muted-foreground">
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
                    className="size-4 shrink-0 text-muted-foreground"
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
            label={t("reviewQueue")}
          />
        </>
      )}
    </div>
  );
}
export function AdminMarketplaceDetail({ id }: { id: string }) {
  const user = useSessionUser();
  const listing = useQuery({
    queryKey: marketplaceKeys.review(user.id, id),
    queryFn: ({ signal }) => getAdminListing(id, signal),
    retry: false,
  });
  return (
    <div className="mx-auto max-w-5xl space-y-6 pb-10">
      <MarketplaceBack href={marketplaceRoutes.admin} />
      {listing.isPending ? (
        <MarketplaceLoading />
      ) : listing.isError && !listing.data ? (
        <MarketplaceError
          error={listing.error}
          retry={() => void listing.refetch()}
        />
      ) : listing.data ? (
        <ReviewListing
          key={id}
          item={listing.data}
          userId={user.id}
          reload={async () => {
            const result = await listing.refetch();
            if (result.isError) throw result.error;
            return result.data;
          }}
        />
      ) : null}
    </div>
  );
}
function ReviewListing({
  item,
  userId,
  reload,
}: {
  item: AdminListing;
  userId: string;
  reload: () => Promise<AdminListing | undefined>;
}) {
  const t = useTranslations("marketplace");
  const fmt = useFormatters();
  const cache = useQueryClient();
  const [snapshot, setSnapshot] = useState(item);
  const [action, setAction] = useState<"approve" | "reject" | "suspend" | null>(
    null,
  );
  const [reloadError, setReloadError] = useState<unknown>(null);
  const [reloading, setReloading] = useState(false);
  const stale = item.updated_at !== snapshot.updated_at;
  async function reloadLatest() {
    setReloading(true);
    setReloadError(null);
    try {
      const result = await reload();
      if (result) setSnapshot(result);
    } catch (error) {
      setReloadError(error);
    } finally {
      setReloading(false);
    }
  }
  return (
    <>
      <header className="space-y-3">
        <ListingStatusBadge status={snapshot.status} />
        <h1 className="break-words text-3xl font-semibold tracking-tight">
          {snapshot.title}
        </h1>
        <p className="text-sm text-muted-foreground">
          {t("by", { name: snapshot.seller.display_name })}
        </p>
      </header>
      {stale && (
        <ErrorState
          variant="compact"
          title={t("staleTitle")}
          description={t("staleDescription")}
          onRetry={() => void reloadLatest()}
          retryLabel={t("reload")}
          isRetrying={reloading}
        />
      )}
      {reloadError !== null && (
        <MarketplaceError
          error={reloadError}
          retry={() => void reloadLatest()}
          fetching={reloading}
        />
      )}
      <div className="grid items-start gap-8 lg:grid-cols-[minmax(0,1fr)_20rem]">
        <div className="min-w-0 space-y-7">
          <ListingCopy
            description={snapshot.description}
            personality={snapshot.default_personality}
          />
          <section className="space-y-2 rounded-xl border bg-muted/20 p-5">
            <h2 className="font-semibold">{t("rights")}</h2>
            <p className="whitespace-pre-wrap break-words text-sm leading-7">
              {snapshot.rights_note}
            </p>
          </section>
          {snapshot.review_note && (
            <section className="space-y-2">
              <h2 className="font-semibold">{t("reviewNote")}</h2>
              <p className="whitespace-pre-wrap break-words text-sm leading-7">
                {snapshot.review_note}
              </p>
            </section>
          )}
          <dl className="grid gap-5 border-t pt-6 text-sm sm:grid-cols-2">
            {[
              { label: t("sellerId"), value: snapshot.seller_id },
              { label: t("assetId"), value: snapshot.asset_id },
              {
                label: t("created"),
                value: fmt.date(snapshot.created_at, "localDateTime"),
              },
              {
                label: t("updatedAt"),
                value: fmt.date(snapshot.updated_at, "localDateTime"),
              },
              ...(snapshot.reviewed_at
                ? [
                    {
                      label: t("reviewedAt"),
                      value: fmt.date(snapshot.reviewed_at, "localDateTime"),
                    },
                  ]
                : []),
              ...(snapshot.reviewed_by
                ? [{ label: t("reviewedBy"), value: snapshot.reviewed_by }]
                : []),
            ].map((row) => (
              <div key={row.label} className="min-w-0 space-y-1">
                <dt className="text-muted-foreground">{row.label}</dt>
                <dd className="break-all">{row.value}</dd>
              </div>
            ))}
          </dl>
        </div>
        <aside className="space-y-5 rounded-xl border bg-card/50 p-5 lg:sticky lg:top-6">
          <ListingPreview path={snapshot.preview_url} title={snapshot.title} />
          <CoinPrice amount={snapshot.price_coins} className="text-xl" />
          <p className="text-sm text-muted-foreground">
            {snapshot.asset_ready ? t("assetReady") : t("assetNotReady")}
          </p>
          {snapshot.status === "pending" && (
            <div className="space-y-2">
              <Button
                className="min-h-11 w-full"
                disabled={!snapshot.asset_ready || stale || reloading}
                onClick={() => setAction("approve")}
              >
                {t("approve")}
              </Button>
              <Button
                variant="outline"
                className="min-h-11 w-full"
                disabled={stale || reloading}
                onClick={() => setAction("reject")}
              >
                {t("reject")}
              </Button>
            </div>
          )}
          {snapshot.status === "active" && (
            <Button
              variant="destructive"
              className="min-h-11 w-full"
              disabled={stale || reloading}
              onClick={() => setAction("suspend")}
            >
              {t("suspend")}
            </Button>
          )}
          {["delisted", "suspended"].includes(snapshot.status) && (
            <p className="text-xs leading-relaxed text-muted-foreground">
              {t("terminalHint")}
            </p>
          )}
        </aside>
      </div>
      {action && (
        <ListingActionDialog
          action={action}
          onClose={() => setAction(null)}
          onReload={() => void reloadLatest()}
          onConfirm={async (reason) => {
            const result = await adminAction(
              snapshot.id,
              action,
              snapshot.updated_at,
              reason,
            );
            setSnapshot(result);
            cache.setQueryData(
              marketplaceKeys.review(userId, result.id),
              result,
            );
            void cache.invalidateQueries({
              queryKey: marketplaceKeys.admin(userId),
            });
            gooeyToast.success(
              t(
                action === "approve"
                  ? "approved"
                  : action === "reject"
                    ? "rejected"
                    : "suspended",
              ),
            );
          }}
        />
      )}
    </>
  );
}
