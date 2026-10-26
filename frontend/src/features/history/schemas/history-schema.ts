import { z } from "zod";
import {
  reportStatusSchema,
  reportTypeSchema,
} from "@/features/report/schemas/report-schema";

// Mirrors GET /interviews (KAN-64, snake_case). It never carries questions, turns, feedback text
// or record fields.
export const HISTORY_PAGE_SIZE = 20;

export const historyEvaluationStatusSchema = z.enum([
  "pending",
  "completed",
  "failed",
]);

export const historyItemSchema = z.object({
  id: z.uuid(),
  type: reportTypeSchema,
  started_at: z.iso.datetime({ offset: true }),
  completed_at: z.iso.datetime({ offset: true }).nullable(),
  evaluation_status: historyEvaluationStatusSchema,
  status: reportStatusSchema,
  score: z.number().int().min(0).max(100).nullable(),
  role_profile_title: z.string().nullable(),
  job_posting_title: z.string().nullable(),
  application_id: z.uuid().nullable(),
});
export type HistoryItem = z.infer<typeof historyItemSchema>;

export const historyPageEnvelopeSchema = z.object({
  success: z.literal(true),
  data: z.object({
    items: z.array(historyItemSchema),
    total: z.number().int().min(0),
    page: z.number().int().min(1),
    page_size: z.number().int().min(1),
  }),
});
export type HistoryPage = z.infer<typeof historyPageEnvelopeSchema>["data"];
