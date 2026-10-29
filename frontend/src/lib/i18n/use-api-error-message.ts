"use client";

import { useCallback } from "react";
import { useTranslations } from "next-intl";
import { apiErrorMessage } from "./api-errors";

// `message(error)` for a failed API call, in the active language. Pass the screen's own
// translated copy as `fallback` to keep action-specific wording for codes the catalog lacks.
export function useApiErrorMessage() {
  const t = useTranslations("errors");
  return useCallback(
    (error: unknown, fallback?: string) => apiErrorMessage(t, error, fallback),
    [t],
  );
}
