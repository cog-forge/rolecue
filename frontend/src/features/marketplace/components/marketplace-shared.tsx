"use client";

import { useState } from "react";
import Link from "next/link";
import Image from "next/image";
import { useTranslations } from "next-intl";
import { ArrowLeft, ArrowUpRight, Coins, ImageOff, Plus } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { LoadingMorph } from "@/components/ui/loading-morph";
import { ErrorState } from "@/components/ui/error-state";
import { useApiErrorMessage } from "@/lib/i18n/use-api-error-message";
import { useFormatters } from "@/lib/i18n/use-formatters";
import { cn } from "@/lib/utils";
import { previewSrc } from "../api/marketplace";
import type { ListingStatus } from "../schemas/marketplace-schema";

export const marketplaceRoutes = {
  catalog: "/marketplace",
  detail: (id: string) => `/marketplace/${encodeURIComponent(id)}`,
  own: "/marketplace/my-listings",
  new: "/marketplace/my-listings/new",
  listing: (id: string) => `/marketplace/my-listings/${encodeURIComponent(id)}`,
  admin: "/admin/marketplace",
  review: (id: string) => `/admin/marketplace/${encodeURIComponent(id)}`,
};

export function MarketplaceShell({
  children,
  own = false,
}: {
  children: React.ReactNode;
  own?: boolean;
}) {
  const t = useTranslations("marketplace");
  return (
    <div className="mx-auto w-full max-w-6xl space-y-8 pb-10">
      <header className="space-y-6">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="max-w-2xl space-y-2">
            <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">
              {own ? t("myListings") : t("title")}
            </h1>
            <p className="text-sm leading-relaxed text-muted-foreground sm:text-base">
              {own ? t("listingsSubtitle") : t("subtitle")}
            </p>
          </div>
          <Button asChild className="min-h-11">
            <Link href={marketplaceRoutes.new}>
              <Plus aria-hidden />
              {t("sell")}
            </Link>
          </Button>
        </div>
        <nav
          aria-label={t("title")}
          className="flex flex-wrap items-center gap-1 border-b"
        >
          {[
            {
              href: marketplaceRoutes.catalog,
              label: t("catalog"),
              active: !own,
            },
            {
              href: marketplaceRoutes.own,
              label: t("myListings"),
              active: own,
            },
          ].map((item) => (
            <Link
              key={item.href}
              href={item.href}
              aria-current={item.active ? "page" : undefined}
              className={cn(
                "min-h-11 border-b-2 px-4 py-3 text-sm font-medium transition-colors motion-reduce:transition-none",
                item.active
                  ? "border-foreground text-foreground"
                  : "border-transparent text-muted-foreground hover:text-foreground",
              )}
            >
              {item.label}
            </Link>
          ))}
          <Link
            href="/avatar-studio"
            className="ml-auto flex min-h-11 items-center gap-1.5 px-3 text-sm text-muted-foreground hover:text-foreground"
          >
            {t("studio")}
            <ArrowUpRight aria-hidden className="size-4" />
          </Link>
        </nav>
      </header>
      {children}
    </div>
  );
}
export function MarketplaceLoading() {
  const t = useTranslations("marketplace");
  return <LoadingMorph label={t("loading")} className="min-h-[25rem]" />;
}
export function MarketplaceError({
  error,
  retry,
  fetching = false,
}: {
  error: unknown;
  retry?: () => void;
  fetching?: boolean;
}) {
  const message = useApiErrorMessage();
  return (
    <ErrorState
      description={message(error)}
      onRetry={retry}
      isRetrying={fetching}
    />
  );
}
export function MarketplaceBack({
  href = marketplaceRoutes.catalog,
}: {
  href?: string;
}) {
  const t = useTranslations("marketplace");
  return (
    <Button asChild variant="ghost" className="-ml-3 min-h-11">
      <Link href={href}>
        <ArrowLeft aria-hidden />
        {href === marketplaceRoutes.admin
          ? t("reviewQueue")
          : href === marketplaceRoutes.own
            ? t("myListings")
            : t("back")}
      </Link>
    </Button>
  );
}
export function ListingPreview({
  path,
  title,
  className,
}: {
  path: string | null;
  title: string;
  className?: string;
}) {
  const t = useTranslations("marketplace");
  const [failedPath, setFailedPath] = useState<string | null>(null);
  const src = previewSrc(path);
  return (
    <div
      className={cn(
        "relative flex aspect-square items-center justify-center overflow-hidden rounded-xl bg-muted/60",
        className,
      )}
    >
      {src && failedPath !== src ? (
        <Image
          src={src}
          alt={title}
          width={640}
          height={640}
          unoptimized
          className="h-full w-full object-contain"
          onError={() => setFailedPath(src)}
        />
      ) : (
        <div className="flex max-w-48 flex-col items-center gap-3 p-6 text-center text-muted-foreground">
          <ImageOff aria-hidden className="size-9 stroke-1" />
          <span className="text-xs leading-relaxed">
            {t("previewUnavailable")}
          </span>
        </div>
      )}
    </div>
  );
}
export function CoinPrice({
  amount,
  className,
}: {
  amount: number;
  className?: string;
}) {
  const fmt = useFormatters();
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 font-semibold tabular-nums",
        className,
      )}
    >
      <Coins aria-hidden className="size-4 text-muted-foreground" />
      {fmt.coins(amount)}
    </span>
  );
}
export function ListingStatusBadge({ status }: { status: ListingStatus }) {
  const t = useTranslations("marketplace.statusLabels");
  return (
    <Badge
      variant="outline"
      className={cn(
        "gap-1.5 rounded-full px-2.5 py-1",
        status === "active" &&
          "border-emerald-600/20 bg-emerald-600/5 text-emerald-700 dark:text-emerald-400",
        status === "pending" &&
          "border-amber-600/20 bg-amber-600/5 text-amber-800 dark:text-amber-300",
        ["rejected", "suspended"].includes(status) &&
          "border-destructive/20 text-destructive",
      )}
    >
      <span className="size-1.5 rounded-full bg-current" aria-hidden />
      {t(status)}
    </Badge>
  );
}
export function ListingCopy({
  description,
  personality,
}: {
  description: string | null;
  personality: string;
}) {
  const t = useTranslations("marketplace");
  return (
    <div className="space-y-7">
      <section className="space-y-2">
        <h2 className="font-semibold">{t("description")}</h2>
        <p className="whitespace-pre-wrap break-words text-sm leading-7 text-muted-foreground">
          {description || t("noDescription")}
        </p>
      </section>
      <section className="space-y-2 border-t pt-6">
        <h2 className="font-semibold">{t("personality")}</h2>
        <p className="whitespace-pre-wrap break-words text-sm leading-7 text-muted-foreground">
          {personality}
        </p>
      </section>
    </div>
  );
}
