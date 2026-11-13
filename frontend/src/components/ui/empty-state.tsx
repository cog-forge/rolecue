"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import {
  FileText,
  Files,
  Link as LinkIcon,
  RotateCcw,
  SearchX,
  type LucideIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export interface EmptyStateAction {
  label: string;
  onClick?: () => void;
  href?: string;
  icon?: LucideIcon;
  variant?:
    "default" | "outline" | "secondary" | "ghost" | "link" | "destructive";
}

export interface EmptyStateProps {
  title?: React.ReactNode;
  description?: React.ReactNode;
  icons?: LucideIcon[];
  action?: EmptyStateAction;
  secondaryAction?: EmptyStateAction;
  variant?: "default" | "dashed" | "card" | "plain";
  isSearch?: boolean;
  searchQuery?: string;
  onClearSearch?: () => void;
  onResetFilters?: () => void;
  className?: string;
  children?: React.ReactNode;
}

export function EmptyState({
  title,
  description,
  icons,
  action,
  secondaryAction,
  variant = "default",
  isSearch = false,
  searchQuery,
  onClearSearch,
  onResetFilters,
  className,
  children,
}: EmptyStateProps) {
  const t = useTranslations("common.emptyState");

  const effectiveTitle = title ?? (isSearch ? t("searchTitle") : t("title"));

  const effectiveDescription =
    description ??
    (isSearch && searchQuery
      ? t("searchDescription", { query: searchQuery })
      : isSearch
        ? t("searchDescription", { query: "" })
        : t("description"));

  // Default icons: if search, use SearchX. Otherwise if icons not provided, empty.
  const displayIcons =
    icons && icons.length > 0
      ? icons
      : isSearch
        ? [SearchX]
        : [LinkIcon, FileText, Files];

  return (
    <div
      role="status"
      aria-label={
        typeof effectiveTitle === "string" ? effectiveTitle : undefined
      }
      className={cn(
        "flex min-h-[16rem] w-full flex-col items-center justify-center p-8 text-center sm:p-12",
        "animate-in fade-in-50 zoom-in-95 duration-200 motion-reduce:animate-none",
        variant === "default" &&
          "rounded-xl border-2 border-dashed border-border bg-transparent",
        variant === "dashed" &&
          "rounded-2xl border border-dashed border-border/80 bg-muted/20",
        variant === "card" && "rounded-2xl border bg-card p-8 shadow-sm",
        variant === "plain" && "border-none bg-transparent",
        className,
      )}
    >
      {/* Icon cluster: stacked layers like in @serafimcloud/empty-state */}
      {displayIcons.length > 0 && (
        <div className="relative mb-5 flex items-center justify-center">
          {displayIcons.length === 1 && (
            <div className="flex size-14 items-center justify-center rounded-2xl border border-border/60 bg-muted/70 text-muted-foreground shadow-xs transition-transform duration-300 hover:scale-105 motion-reduce:transition-none">
              {(() => {
                const Icon = displayIcons[0];
                return (
                  <Icon className="size-7 stroke-[1.75]" aria-hidden="true" />
                );
              })()}
            </div>
          )}

          {displayIcons.length === 2 && (
            <div className="relative flex h-14 w-20 items-center justify-center">
              <div className="absolute -left-1 flex size-11 items-center justify-center -rotate-6 rounded-xl border border-border/50 bg-muted/50 text-muted-foreground/70">
                {(() => {
                  const Icon = displayIcons[1];
                  return (
                    <Icon className="size-5 stroke-[1.75]" aria-hidden="true" />
                  );
                })()}
              </div>
              <div className="relative z-10 flex size-12 items-center justify-center rounded-xl border border-border/70 bg-card text-foreground shadow-xs">
                {(() => {
                  const Icon = displayIcons[0];
                  return (
                    <Icon className="size-6 stroke-[1.75]" aria-hidden="true" />
                  );
                })()}
              </div>
            </div>
          )}

          {displayIcons.length >= 3 && (
            <div className="relative flex h-14 w-28 items-center justify-center">
              <div className="absolute -left-2 flex size-11 items-center justify-center -rotate-12 rounded-xl border border-border/50 bg-muted/40 text-muted-foreground/60">
                {(() => {
                  const Icon = displayIcons[1];
                  return (
                    <Icon className="size-5 stroke-[1.75]" aria-hidden="true" />
                  );
                })()}
              </div>
              <div className="absolute -right-2 flex size-11 items-center justify-center rotate-12 rounded-xl border border-border/50 bg-muted/40 text-muted-foreground/60">
                {(() => {
                  const Icon = displayIcons[2];
                  return (
                    <Icon className="size-5 stroke-[1.75]" aria-hidden="true" />
                  );
                })()}
              </div>
              <div className="relative z-10 flex size-13 items-center justify-center rounded-2xl border border-border/70 bg-card text-foreground shadow-xs">
                {(() => {
                  const Icon = displayIcons[0];
                  return (
                    <Icon className="size-6 stroke-[1.75]" aria-hidden="true" />
                  );
                })()}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Typography */}
      <div className="max-w-md space-y-2">
        <h3 className="text-base font-semibold tracking-tight text-foreground sm:text-lg">
          {effectiveTitle}
        </h3>
        {effectiveDescription && (
          <p className="text-sm leading-relaxed text-muted-foreground">
            {effectiveDescription}
          </p>
        )}
      </div>

      {/* Actions */}
      <div className="mt-6 flex flex-wrap items-center justify-center gap-3">
        {onClearSearch && (
          <Button
            type="button"
            variant="outline"
            className="h-10 min-w-28 rounded-xl font-medium"
            onClick={onClearSearch}
          >
            <SearchX className="size-4" aria-hidden="true" />
            {t("clearSearch")}
          </Button>
        )}

        {onResetFilters && (
          <Button
            type="button"
            variant="outline"
            className="h-10 min-w-28 rounded-xl font-medium"
            onClick={onResetFilters}
          >
            <RotateCcw className="size-4" aria-hidden="true" />
            {t("resetFilters")}
          </Button>
        )}

        {action &&
          (action.href ? (
            <Button
              asChild
              variant={action.variant ?? "default"}
              className="h-10 min-w-28 rounded-xl font-medium"
            >
              <Link href={action.href}>
                {action.icon && (
                  <action.icon className="size-4" aria-hidden="true" />
                )}
                {action.label}
              </Link>
            </Button>
          ) : (
            <Button
              type="button"
              variant={action.variant ?? "default"}
              className="h-10 min-w-28 rounded-xl font-medium"
              onClick={action.onClick}
            >
              {action.icon && (
                <action.icon className="size-4" aria-hidden="true" />
              )}
              {action.label}
            </Button>
          ))}

        {secondaryAction &&
          (secondaryAction.href ? (
            <Button
              asChild
              variant={secondaryAction.variant ?? "outline"}
              className="h-10 min-w-28 rounded-xl font-medium"
            >
              <Link href={secondaryAction.href}>
                {secondaryAction.icon && (
                  <secondaryAction.icon className="size-4" aria-hidden="true" />
                )}
                {secondaryAction.label}
              </Link>
            </Button>
          ) : (
            <Button
              type="button"
              variant={secondaryAction.variant ?? "outline"}
              className="h-10 min-w-28 rounded-xl font-medium"
              onClick={secondaryAction.onClick}
            >
              {secondaryAction.icon && (
                <secondaryAction.icon className="size-4" aria-hidden="true" />
              )}
              {secondaryAction.label}
            </Button>
          ))}
      </div>

      {children && <div className="mt-6 w-full">{children}</div>}
    </div>
  );
}
