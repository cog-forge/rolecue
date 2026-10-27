import { apiClient } from "@/lib/api/client";
import { reportEnvelopeSchema } from "../schemas/report-schema";

// Scoped to the user: a report is visible only to the users the Go API lets read it.
export const reportKeys = {
  all: (userId: string) => ["interview-reports", userId] as const,
  detail: (userId: string, interviewId: string) =>
    [...reportKeys.all(userId), interviewId] as const,
};

export async function getInterviewReport(
  interviewId: string,
  signal?: AbortSignal,
) {
  const response = await apiClient.get<unknown>(
    `/interviews/${encodeURIComponent(interviewId)}/report`,
    { signal },
  );
  return reportEnvelopeSchema.parse(response.data).data;
}
