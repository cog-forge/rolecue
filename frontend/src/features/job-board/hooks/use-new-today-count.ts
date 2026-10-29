"use client";

import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { getNewPostingCount, jobBoardKeys } from "../api/job-board";
import { publicationDay } from "../utils/format";

// Postings first published in the current UTC day. The Job Board header and the candidate sidebar
// share one query, so both show the same number from a single request.
export function useNewTodayCount(enabled = true) {
  const user = useSessionUser();
  const [day, setDay] = useState(publicationDay);
  useEffect(() => {
    const nextDay = window.setTimeout(
      () => setDay(publicationDay()),
      Math.max(1, Date.parse(day.to) - Date.now() + 10),
    );
    return () => window.clearTimeout(nextDay);
  }, [day]);
  return useQuery({
    queryKey: jobBoardKeys.newToday(user.id, day.from),
    queryFn: ({ signal }) => getNewPostingCount(day.from, day.to, signal),
    enabled,
    refetchOnWindowFocus: true,
    retry: false,
  });
}
