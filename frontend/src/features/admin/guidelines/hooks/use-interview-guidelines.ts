"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { readApiError } from "@/lib/i18n/api-errors";
import {
  getGuidelines,
  guidelineKeys,
  saveGuidelines,
} from "../api/guidelines";
import type { GuidelineValues } from "../schemas/guidelines-schema";

// What the last save attempt did, so the page can say it in words. A conflict means someone else saved
// first: the latest version was loaded and this Admin's edits were not saved.
export type GuidelineOutcome =
  | { kind: "saved"; version: number }
  | { kind: "reset"; version: number }
  | { kind: "conflict" };

export function isAccessDenied(error: unknown) {
  return [401, 403].includes(readApiError(error).status);
}

export function isVersionConflict(error: unknown) {
  const { status, code } = readApiError(error);
  return status === 409 && code === "GUIDELINE_VERSION_CONFLICT";
}

// Loads the guidelines and saves a new version. Saving never retries by itself: a repeated PUT after a
// timeout could land on a version that already moved on, so the Admin decides.
export function useInterviewGuidelines() {
  const user = useSessionUser();
  const cache = useQueryClient();
  const key = guidelineKeys.all(user.id);
  const [outcome, setOutcome] = useState<GuidelineOutcome | null>(null);
  const query = useQuery({
    queryKey: key,
    queryFn: ({ signal }) => getGuidelines(signal),
    refetchOnMount: "always",
    retry: false,
  });
  const save = useMutation({
    mutationFn: ({
      expectedVersion,
      values,
    }: {
      expectedVersion: number;
      values: GuidelineValues;
      reset?: boolean;
    }) => saveGuidelines(expectedVersion, values),
    retry: false,
    onMutate: () => setOutcome(null),
    onSuccess: (saved, variables) => {
      cache.setQueryData(key, saved);
      setOutcome({
        kind: variables.reset ? "reset" : "saved",
        version: saved.version,
      });
    },
    onError: (error) => {
      if (!isVersionConflict(error)) return;
      setOutcome({ kind: "conflict" });
      void cache.invalidateQueries({ queryKey: key });
    },
  });
  const denied = isAccessDenied(query.error) || isAccessDenied(save.error);
  useEffect(() => {
    // A role or lock change shows up as 401/403; let the session gate re-check access.
    if (denied) void cache.invalidateQueries({ queryKey: ["auth", "me"] });
  }, [cache, denied]);

  return {
    query,
    save,
    denied,
    outcome,
    dismissOutcome: () => setOutcome(null),
  };
}
