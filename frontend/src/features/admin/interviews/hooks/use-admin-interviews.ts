"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useSessionUser } from "@/features/auth/components/session-gate";
import {
  adminInterviewKeys,
  initialInterviewQuery,
  isAdminInterviewAccessError,
  listAdminInterviews,
  type AdminInterviewQuery,
} from "../api/interviews";
import type { InterviewFilterValues } from "../schemas/interview-filters";

// Owns the list query, its filters and paging for the Admin interview sessions screen.
export function useAdminInterviews() {
  const viewer = useSessionUser();
  const [query, setQuery] = useState<AdminInterviewQuery>(
    initialInterviewQuery,
  );
  const list = useQuery({
    queryKey: adminInterviewKeys.list(viewer.id, query),
    queryFn: ({ signal }) => listAdminInterviews(query, signal),
    enabled: viewer.role === "admin",
    staleTime: 0,
    refetchOnMount: "always",
    retry: false,
  });
  return {
    list,
    query,
    denied: isAdminInterviewAccessError(list.error),
    applyFilters: (values: InterviewFilterValues) =>
      setQuery({
        page: 1,
        type: values.type === "all" ? null : values.type,
        candidateId: values.candidateId || null,
        jobPostingId: values.jobPostingId || null,
      }),
    changePage: (page: number) => setQuery((current) => ({ ...current, page })),
  };
}
