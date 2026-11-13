"use client";
import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";

export interface LoadingMorphProps {
  label?: string;
  size?: "sm" | "md" | "lg" | "fullscreen";
  className?: string;
  showText?: boolean;
  role?: string | null;
}
export function LoadingMorph({
  label,
  size = "md",
  className,
  showText = true,
  role = "status",
}: LoadingMorphProps) {
  const t = useTranslations("common");
  const effectiveLabel = label ?? t("loadingPage");
  return (
    <div
      role={role ?? undefined}
      aria-label={effectiveLabel}
      className={cn(
        "flex w-full flex-col items-center justify-center gap-3 text-center",
        size === "fullscreen"
          ? "fixed inset-0 z-50 bg-background"
          : "min-h-56 py-10",
        className,
      )}
    >
      <div
        aria-hidden="true"
        className={cn(
          "relative",
          size === "sm" ? "size-16" : size === "md" ? "size-24" : "size-32",
        )}
      >
        <div className="absolute inset-0 flex items-center justify-center">
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
