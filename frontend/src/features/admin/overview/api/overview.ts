import { isAxiosError } from "axios";
import { apiClient } from "@/lib/api/client";
import {
  overviewEnvelopeSchema,
  type Overview,
  type OverviewDays,
} from "../schemas/overview-schema";

export const overviewKeys = {
  all: ["admin", "overview"] as const,
  overview: (userId: string, days: OverviewDays) =>
    [...overviewKeys.all, userId, days] as const,
};

export async function getOverview(
  days: OverviewDays,
  signal?: AbortSignal,
): Promise<Overview> {
  const response = await apiClient.get<unknown>("/admin/overview", {
    params: { days },
    signal,
  });
  return overviewEnvelopeSchema.parse(response.data).data;
}

export function isAccessDenied(error: unknown) {
  return (
    isAxiosError(error) && [401, 403].includes(error.response?.status ?? 0)
  );
}
