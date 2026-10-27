import { z } from "zod";
import { isAxiosError } from "axios";
import { apiClient } from "@/lib/api/client";

export const ADMIN_INTERVIEWS_PAGE_SIZE = 20;

export const interviewTypes = ["practice", "recruitment"] as const;
export const interviewTypeSchema = z.enum(interviewTypes);
export type InterviewType = z.infer<typeof interviewTypeSchema>;

// Allow-list contracts: zod strips any field not named here, so transcripts, answers
// and recording paths never reach the screen even if the backend ever sends them.
const rowSchema = z.object({
  id: z.uuid(),
  type: interviewTypeSchema,
  owner: z.object({
    id: z.uuid(),
    name: z.string(),
    email: z.email(),
  }),
  job_posting: z.object({ id: z.uuid(), title: z.string() }).nullable(),
  score: z.number().int().nullable(),
  has_report: z.boolean(),
  has_recording: z.boolean(),
});

const detailSchema = rowSchema.extend({
  role_profile: z.object({ id: z.uuid(), title: z.string() }).nullable(),
  application_id: z.uuid().nullable(),
  avatar: z.object({ id: z.uuid(), name: z.string() }).nullable(),
  voice: z.object({ id: z.uuid(), name: z.string() }).nullable(),
  question_count: z.number().int().nonnegative(),
  turn_count: z.number().int().nonnegative(),
});

const listSchema = z.object({
  items: z.array(rowSchema),
  total: z.number().int().nonnegative(),
  page: z.number().int().positive(),
  page_size: z.number().int().positive(),
});

export type AdminInterviewRow = z.infer<typeof rowSchema>;
export type AdminInterviewDetail = z.infer<typeof detailSchema>;
export type AdminInterviewList = z.infer<typeof listSchema>;

export type AdminInterviewQuery = {
  page: number;
  type: InterviewType | null;
  candidateId: string | null;
  jobPostingId: string | null;
};

export const initialInterviewQuery: AdminInterviewQuery = {
  page: 1,
  type: null,
  candidateId: null,
  jobPostingId: null,
};

export const adminInterviewKeys = {
  viewer: (viewerId: string) => ["admin-interviews", viewerId] as const,
  lists: (viewerId: string) =>
    [...adminInterviewKeys.viewer(viewerId), "list"] as const,
  list: (viewerId: string, query: AdminInterviewQuery) =>
    [...adminInterviewKeys.lists(viewerId), query] as const,
  detail: (viewerId: string, id: string) =>
    [...adminInterviewKeys.viewer(viewerId), "detail", id] as const,
};

export function isAdminInterviewAccessError(error: unknown) {
  return (
    isAxiosError(error) && [401, 403].includes(error.response?.status ?? 0)
  );
}

export function adminInterviewErrorCode(error: unknown) {
  if (!isAxiosError(error)) return null;
  const parsed = z
    .object({ error: z.object({ code: z.string() }) })
    .safeParse(error.response?.data);
  return parsed.success ? parsed.data.error.code : null;
}

export async function listAdminInterviews(
  query: AdminInterviewQuery,
  signal?: AbortSignal,
) {
  const response = await apiClient.get<unknown>("/admin/interviews", {
    params: {
      page: query.page,
      page_size: ADMIN_INTERVIEWS_PAGE_SIZE,
      ...(query.type ? { type: query.type } : {}),
      ...(query.candidateId ? { candidate_id: query.candidateId } : {}),
      ...(query.jobPostingId ? { job_posting_id: query.jobPostingId } : {}),
    },
    signal,
  });
  return z
    .object({ success: z.literal(true), data: listSchema })
    .parse(response.data).data;
}

export async function getAdminInterview(id: string, signal?: AbortSignal) {
  const response = await apiClient.get<unknown>(
    `/admin/interviews/${z.uuid().parse(id)}`,
    { signal },
  );
  return z
    .object({ success: z.literal(true), data: detailSchema })
    .parse(response.data).data;
}
