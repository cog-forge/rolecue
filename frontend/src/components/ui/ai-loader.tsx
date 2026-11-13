"use client";
import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";

export interface AiLoaderProps {
  stage?: "thinking" | "generating" | "evaluating" | "processing";
  title?: string;
  description?: string;
  size?: "sm" | "md" | "lg";
  className?: string;
}
export function AiLoader({
  stage = "thinking",
  title,
  description,
  size = "md",
  className,
}: AiLoaderProps) {
  const t = useTranslations("common.aiLoader");
  const text = title ?? t(stage);
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
        <span className="sr-only">{text}</span>
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
