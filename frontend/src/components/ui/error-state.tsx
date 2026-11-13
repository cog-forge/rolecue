"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import {
  CircleAlert,
  ArrowLeft,
  Home,
  RefreshCw,
  type LucideIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export interface ErrorStateProps {
  title?: React.ReactNode;
  heading?: React.ReactNode;
  headingLevel?: 1 | 2 | 3;
  description?: React.ReactNode;
  error?: Error | unknown;
  statusCode?: number;
  onRetry?: () => void;
  isRetrying?: boolean;
  retryLabel?: React.ReactNode;
  backHref?: string;
  homeHref?: string;
  onBack?: () => void;
  variant?: "card" | "inline" | "full-page" | "dashed";
  icon?: LucideIcon;
  className?: string;
  children?: React.ReactNode;
}

export function ErrorState({
  title,
  heading,
  headingLevel = 2,
  description,
  statusCode,
  onRetry,
  isRetrying = false,
  retryLabel,
  backHref,
  homeHref,
  onBack,
  variant = "card",
  icon: Icon = CircleAlert,
  className,
  children,
}: ErrorStateProps) {
  const t = useTranslations("common.errorState");
  const Heading = `h${headingLevel}` as "h1" | "h2" | "h3";

  const effectiveTitle = title ?? t("title");

  const effectiveDescription =
    description ??
    (statusCode === 404
      ? t("status404")
      : statusCode === 403
        ? t("status403")
        : statusCode === 500
          ? t("status500")
          : t("description"));

  return (
    <div
      role="alert"
      aria-live="assertive"
      className={cn(
        "flex w-full flex-col items-center justify-center text-center",
        "animate-in fade-in-50 zoom-in-95 duration-200 motion-reduce:animate-none",
        variant === "card" && "min-h-[22rem] border bg-transparent p-8 sm:p-10",
        variant === "full-page" &&
          "min-h-[60vh] max-w-xl mx-auto border bg-transparent p-8 sm:p-12",
        variant === "inline" && "min-h-[20rem] border bg-transparent p-8",
        variant === "dashed" &&
          "min-h-[14rem] rounded-2xl border border-dashed border-destructive/30 bg-destructive/[0.02] p-8",
        className,
      )}
    >
      {/* Icon with soft destructive halo */}
      <div className="relative mb-4 flex items-center justify-center">
        <div className="flex size-11 items-center justify-center rounded-xl bg-destructive/10 text-destructive">
          <Icon className="size-4 stroke-[1.8]" aria-hidden="true" />
        </div>
      </div>

      {/* Typography */}
      <div className={cn("max-w-xs space-y-2")}>
        <div className="flex items-center justify-center gap-2">
          {heading ?? (
            <Heading className="text-base font-semibold tracking-tight text-foreground sm:text-lg">
              {effectiveTitle}
            </Heading>
          )}
          {statusCode && (
            <span className="inline-flex items-center rounded-md border border-destructive/30 bg-destructive/10 px-2 py-0.5 text-xs font-semibold text-destructive">
              {statusCode}
            </span>
          )}
        </div>

        {effectiveDescription && (
          <p className="text-sm leading-relaxed text-muted-foreground">
            {effectiveDescription}
          </p>
        )}
      </div>

      {/* Actions */}
      <div className="mt-6 flex flex-wrap items-center justify-center gap-3">
        {onRetry && (
          <Button
            type="button"
            variant="default"
            className="h-10 min-w-28 font-medium"
            disabled={isRetrying}
            onClick={onRetry}
          >
            <RefreshCw
              className={cn("size-4", isRetrying && "animate-spin")}
              aria-hidden="true"
            />
            {retryLabel ?? t("retry")}
          </Button>
        )}

        {onBack && (
          <Button
            type="button"
            variant="ghost"
            className="h-10 rounded-xl font-medium"
            onClick={onBack}
          >
            <ArrowLeft className="size-4" aria-hidden="true" />
            {t("back")}
          </Button>
        )}

        {backHref && (
          <Button
            asChild
            variant="ghost"
            className="h-10 rounded-xl font-medium"
          >
            <Link href={backHref}>
              <ArrowLeft className="size-4" aria-hidden="true" />
              {t("back")}
            </Link>
          </Button>
        )}

        {homeHref && (
          <Button
            asChild
            variant="outline"
            className="h-10 rounded-xl font-medium"
          >
            <Link href={homeHref}>
              <Home className="size-4" aria-hidden="true" />
              {t("home")}
            </Link>
          </Button>
        )}
      </div>

      {children && <div className="mt-6 w-full">{children}</div>}
    </div>
  );
}
