"use client";

import { useCallback } from "react";
import { useTranslations } from "next-intl";
import { roleErrorMessage } from "./role-error";

export function useRoleError() {
  const t = useTranslations("candidate.roleProfiles.errors");
  const errors = useTranslations("errors");
  return useCallback(
    (error: unknown) => roleErrorMessage(t, errors, error),
    [t, errors],
  );
}
