import { z } from "zod";
import { isAxiosError } from "axios";
import { apiClient } from "@/lib/api/client";

export const ACCESS_AUDIT_PAGE_SIZE = 20;

export const accessActions = ["recording_view", "transcript_view"] as const;

// Mirrors domain.AccessAuditPage (KAN-296). IDs and times only: the audit never carries the content that
// was read.
const entrySchema = z.object({
  id: z.uuid(),
  recruiter_id: z.uuid(),
  application_id: z.uuid(),
  interview_id: z.uuid(),
  action: z.enum(accessActions),
  created_at: z.iso.datetime({ offset: true }),
});

const pageSchema = z.object({
  items: z.array(entrySchema),
  total: z.number().int().nonnegative(),
  page: z.number().int().positive(),
  page_size: z.number().int().positive(),
});

export type AccessAuditEntry = z.infer<typeof entrySchema>;
export type AccessAuditPage = z.infer<typeof pageSchema>;

export type AccessAuditQuery = {
  page: number;
  recruiterId: string | null;
  applicationId: string | null;
};

export const initialAccessAuditQuery: AccessAuditQuery = {
  page: 1,
  recruiterId: null,
  applicationId: null,
};

export const accessAuditKeys = {
  viewer: (viewerId: string) => ["admin-access-audit", viewerId] as const,
  list: (viewerId: string, query: AccessAuditQuery) =>
    [...accessAuditKeys.viewer(viewerId), "list", query] as const,
};

export function isAccessAuditDenied(error: unknown) {
  return (
    isAxiosError(error) && [401, 403].includes(error.response?.status ?? 0)
  );
}

export async function listAccessAudit(
  query: AccessAuditQuery,
  signal?: AbortSignal,
) {
  const response = await apiClient.get<unknown>("/admin/access-audit", {
    params: {
      page: query.page,
      page_size: ACCESS_AUDIT_PAGE_SIZE,
      ...(query.recruiterId ? { recruiter_id: query.recruiterId } : {}),
      ...(query.applicationId ? { application_id: query.applicationId } : {}),
    },
    signal,
  });
  return z
    .object({ success: z.literal(true), data: pageSchema })
    .parse(response.data).data;
}
