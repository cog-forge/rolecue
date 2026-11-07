import { apiClient } from "@/lib/api/client";
import { z } from "zod";

// Download links inside the export live this long (the API's ExportLinkTTL).
export const EXPORT_LINK_MINUTES = 15;

// The word the API accepts as confirmation of an erasure.
export const ERASURE_CONFIRMATION = "DELETE";

// Only the envelope is checked here: the file the user saves is the API's own JSON, kept as it came.
const exportEnvelopeSchema = z.object({
  data: z.looseObject({ link_ttl_seconds: z.number() }),
});

export async function fetchMyDataExport(signal?: AbortSignal) {
  const response = await apiClient.get<unknown>("/account/export", { signal });
  return exportEnvelopeSchema.parse(response.data).data;
}

export async function deleteMyAccount() {
  await apiClient.delete("/account", {
    data: { confirm: ERASURE_CONFIRMATION },
  });
}
