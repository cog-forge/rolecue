import { apiClient } from "@/lib/api/client";
import {
  guidelinesEnvelopeSchema,
  type GuidelineValues,
} from "../schemas/guidelines-schema";

export const guidelineKeys = {
  all: (userId: string) => ["admin-interview-guidelines", userId] as const,
};

export async function getGuidelines(signal?: AbortSignal) {
  const response = await apiClient.get<unknown>("/admin/interview-guidelines", {
    signal,
  });
  return guidelinesEnvelopeSchema.parse(response.data).data;
}

// expectedVersion is the version the Admin loaded (0 before the first save). Any other current version is
// a 409 GUIDELINE_VERSION_CONFLICT and nothing is written. The save appends a new version.
export async function saveGuidelines(
  expectedVersion: number,
  values: GuidelineValues,
) {
  const response = await apiClient.put<unknown>("/admin/interview-guidelines", {
    expected_version: expectedVersion,
    values,
  });
  return guidelinesEnvelopeSchema.parse(response.data).data;
}
