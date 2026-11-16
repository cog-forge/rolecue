"use client";
import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getRoadmap, roadmapKeys, type Roadmap } from "../api/roadmap";

export function roadmapPollInterval(
  data: Roadmap | undefined,
  elapsed: number,
): number | false {
  if (data?.generation_status === "generating")
    return elapsed >= 300_000 ? false : elapsed < 30_000 ? 3_000 : 5_000;
  if (data?.update_state === "updating" && elapsed < 120_000) return 10_000;
  return false;
}
export function useRoadmap(userId: string, profileId: string, paused = false) {
  const phase = useRef({ name: "", since: 0 });
  const [generationTimedOut, setGenerationTimedOut] = useState(false);
  const query = useQuery({
    queryKey: roadmapKeys.detail(userId, profileId),
    queryFn: ({ signal }) => getRoadmap(profileId, signal),
    enabled: Boolean(profileId) && !paused,
    retry: false,
    staleTime: 0,
    refetchOnMount: "always",
    refetchOnWindowFocus: "always",
    refetchIntervalInBackground: false,
    refetchInterval: (current) => {
      if (paused || current.state.error) return false;
      const data = current.state.data;
      const name =
        data?.generation_status === "generating"
          ? "generating"
          : (data?.update_state ?? "");
      if (phase.current.name !== name)
        phase.current = { name, since: Date.now() };
      return roadmapPollInterval(data, Date.now() - phase.current.since);
    },
  });
  const generating = query.data?.generation_status === "generating";
  useEffect(() => {
    const since = phase.current.since;
    const timer = setTimeout(
      () => setGenerationTimedOut(Boolean(generating)),
      generating ? Math.max(0, 300_000 - (Date.now() - since)) : 0,
    );
    return () => clearTimeout(timer);
  }, [generating]);
  return { ...query, generationTimedOut: generating && generationTimedOut };
}
