"use client";

import { useTranslations } from "next-intl";

import { useId, type ReactNode } from "react";
import { ErrorState } from "@/components/ui/error-state";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

// Shared shadcn frame and transitions.dev skeleton reveal for every dashboard widget.
export function DashboardCard({
  title,
  children,
  className,
  action,
  tone,
  isLoading = false,
  compact = false,
}: {
  title: string;
  children: ReactNode;
  className?: string;
  action?: ReactNode;
  tone?: "accent";
  isLoading?: boolean;
  compact?: boolean;
}) {
  const titleId = useId();
  const t = useTranslations("candidate.dashboard");
  return (
    <section
      aria-labelledby={titleId}
      aria-busy={isLoading}
      className={cn("min-w-0", className)}
    >
      <Card
        className={cn(
          "h-full gap-5 overflow-hidden rounded-xl py-5 shadow-none [--card-spacing:--spacing(5)] sm:py-6 sm:[--card-spacing:--spacing(6)]",
          tone === "accent" && "border-rolecue-brand/15 bg-rolecue-brand/5",
        )}
      >
        <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-3">
          <CardTitle>
            <h2 id={titleId} className="text-base font-semibold">
              {title}
            </h2>
          </CardTitle>
          {action}
        </CardHeader>
        <CardContent>
          <div
            className={cn(
              "t-skel",
              compact ? "min-h-20" : "min-h-32",
              !isLoading && "is-revealed",
            )}
          >
            <div
              className="t-skel-skeleton"
              aria-hidden={!isLoading}
              inert={!isLoading}
            >
              <CardLoading label={t("loadingCard", { title })} />
            </div>
            <div
              className="t-skel-content relative! space-y-4"
              aria-hidden={isLoading}
              inert={isLoading}
            >
              {children}
            </div>
          </div>
        </CardContent>
      </Card>
    </section>
  );
}

export function CardLoading({ label }: { label: string }) {
  return (
    <div role="status" aria-label={label} className="space-y-3">
      <Skeleton className="h-8 w-24" />
      <Skeleton className="h-4 w-full" />
      <Skeleton className="h-4 w-3/4" />
      <span className="sr-only">{label}</span>
    </div>
  );
}

export function CardError({
  message,
  disabled,
  onRetry,
}: {
  message: string;
  disabled: boolean;
  onRetry: () => void;
}) {
  const t = useTranslations("candidate.dashboard");
  return (
    <ErrorState
      variant="inline"
      description={message}
      isRetrying={disabled}
      onRetry={onRetry}
      retryLabel={t("retry")}
    />
  );
}
