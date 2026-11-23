"use client";
import { useTranslations } from "next-intl";
import { Check, LoaderCircle } from "lucide-react";
import { Button } from "@/components/ui/button";

// A quiet line under the choices: saving, saved, or what failed with a retry. Always rendered so a
// screen reader hears the change without the layout jumping.
export function SaveStatus({
  state,
  onRetry,
}: {
  state: "idle" | "saving" | "saved" | "error";
  onRetry: () => void;
}) {
  const t = useTranslations("candidate.setup.saveStatus");
  return (
    <div className="flex min-h-11 flex-wrap items-center gap-2 text-sm text-muted-foreground">
      <span role="status" className="inline-flex items-center gap-1.5">
        {state === "saving" && (
          <>
            <LoaderCircle
              aria-hidden="true"
              className="size-4 animate-spin motion-reduce:animate-none"
            />
            {t("saving")}
          </>
        )}
        {state === "saved" && (
          <>
            <Check aria-hidden="true" className="size-4" />
            {t("saved")}
          </>
        )}
      </span>
      {state === "error" && (
        <>
          <span role="alert" className="text-destructive">
            {t("failed")}
          </span>
          <Button
            type="button"
            variant="outline"
            className="min-h-11"
            onClick={onRetry}
          >
            {t("retry")}
          </Button>
        </>
      )}
    </div>
  );
}
