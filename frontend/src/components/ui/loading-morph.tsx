"use client";
import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";

export interface LoadingMorphProps {
  label?: string;
  size?: "sm" | "md" | "lg" | "fullscreen";
  className?: string;
  showText?: boolean;
  role?: string | null;
  variant?: "page" | "compact";
}
export function LoadingMorph({
  label,
  size = "md",
  className,
  showText = true,
  role = "status",
  variant = "page",
}: LoadingMorphProps) {
  const t = useTranslations("common");
  const effectiveLabel = label ?? t("loadingPage");
  return (
    <div
      role={role ?? undefined}
      aria-label={effectiveLabel}
      aria-live={role === "status" ? "polite" : undefined}
      className={cn(
        "flex w-full flex-col items-center justify-center gap-3 text-center",
        size === "fullscreen"
          ? "fixed inset-0 z-50 bg-background"
          : variant === "compact"
            ? "flex-row justify-start py-3 text-left"
            : "min-h-56 py-10",
        className,
      )}
    >
      <div
        aria-hidden="true"
        className={cn(
          "relative shrink-0",
          variant === "compact"
            ? "size-10"
            : size === "sm"
              ? "size-16"
              : size === "md"
                ? "size-24"
                : "size-32",
        )}
      >
        <div
          className={cn(
            "absolute inset-0 flex items-center justify-center",
            variant === "compact" && "scale-50",
          )}
        >
          {[0, 1, 2, 3].map((i) => (
            <div
              key={i}
              className="morph-dot absolute size-4"
              style={{
                animationName: `morph-${i}`,
                animationDelay: `${i * 0.2}s`,
              }}
            />
          ))}
        </div>
      </div>
      {showText && (
        <p className="text-sm text-muted-foreground">{effectiveLabel}</p>
      )}
    </div>
  );
}
