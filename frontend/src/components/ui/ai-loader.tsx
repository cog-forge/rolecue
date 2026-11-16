"use client";
import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";

export interface AiLoaderProps {
  stage?: "thinking" | "generating" | "evaluating" | "processing";
  title?: string;
  description?: string;
  size?: "sm" | "md" | "lg";
  className?: string;
  variant?: "panel" | "compact";
  headingLevel?: 2 | 3;
}
export function AiLoader({
  stage = "thinking",
  title,
  description,
  size = "md",
  className,
  variant = "panel",
  headingLevel,
}: AiLoaderProps) {
  const t = useTranslations("common.aiLoader");
  const text = title ?? t(stage);
  const Title = headingLevel === 2 ? "h2" : headingLevel === 3 ? "h3" : "span";
  if (variant === "compact")
    return (
      <div
        role="status"
        aria-live="polite"
        className={cn(
          "flex w-full items-center gap-4 rounded-xl border bg-muted/30 p-4",
          className,
        )}
      >
        <div
          aria-hidden="true"
          className="relative size-12 shrink-0 overflow-hidden rounded-full bg-slate-950"
        >
          <div className="ai-loader-circle absolute inset-0 rounded-full" />
        </div>
        <div className="min-w-0 space-y-1">
          <Title className="block text-sm font-medium">{text}</Title>
          {description && (
            <p className="text-sm leading-relaxed text-muted-foreground">
              {description}
            </p>
          )}
        </div>
      </div>
    );
  return (
    <div
      role="status"
      aria-live="polite"
      className={cn(
        "flex min-h-64 w-full flex-col items-center justify-center gap-5 rounded-xl bg-linear-to-b from-[#1a3379] via-[#0f172a] to-black px-6 py-8 text-center text-white",
        className,
      )}
    >
      <div
        className={cn(
          "relative flex shrink-0 items-center justify-center select-none",
          size === "sm" ? "size-36" : size === "lg" ? "size-56" : "size-48",
        )}
      >
        <div
          aria-hidden="true"
          className="ai-loader-circle absolute inset-0 rounded-full"
        />
        <Title className="sr-only">{text}</Title>
        <span
          aria-hidden="true"
          className="z-10 max-w-[75%] text-sm font-medium leading-snug"
        >
          {text.split(/(\s+)/).map((word, wordIndex) => (
            <span key={wordIndex} className="inline-block whitespace-pre">
              {Array.from(word).map((letter, letterIndex) => (
                <span
                  key={letterIndex}
                  className="ai-loader-letter inline-block"
                  style={{
                    animationDelay: `${(wordIndex * 4 + letterIndex) * 0.1}s`,
                  }}
                >
                  {letter}
                </span>
              ))}
            </span>
          ))}
        </span>
      </div>
      {description && (
        <p className="max-w-sm text-sm text-white/80">{description}</p>
      )}
    </div>
  );
}
