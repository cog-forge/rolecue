"use client";

import { useTranslations } from "next-intl";

import { useId, type ReactNode } from "react";
import { ErrorState } from "@/components/ui/error-state";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

// Shared frame for one dashboard card. Each card owns its own query, so a failing card shows its
// error state while the others keep working.
export function DashboardCard({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  const titleId = useId();
  return (
    <section aria-labelledby={titleId} className="min-w-0">
      <Card className="h-full">
        <CardHeader>
          <CardTitle>
            <h2 id={titleId} className="text-base font-semibold">
              {title}
            </h2>
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">{children}</CardContent>
      </Card>
    </section>
  );
}

export function CardLoading({ label }: { label: string }) {
  return (
    <div role="status" className="space-y-3">
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
