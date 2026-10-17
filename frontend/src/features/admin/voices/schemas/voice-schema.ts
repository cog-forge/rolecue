import { z } from "zod";

// Mirrors api/internal/features/admin/voicecatalog/domain (snake_case JSON).
export const voiceSchema = z.object({
  id: z.uuid(),
  name: z.string(),
  is_active: z.boolean(),
  interview_count: z.number().int().nonnegative(),
  job_posting_count: z.number().int().nonnegative(),
  in_use: z.boolean(),
});
export type Voice = z.infer<typeof voiceSchema>;

export const voicePageEnvelopeSchema = z.object({
  success: z.literal(true),
  data: z.object({
    items: z.array(voiceSchema),
    pagination: z.object({
      limit: z.number().int(),
      offset: z.number().int(),
      total: z.number().int(),
    }),
  }),
});
export type VoicePage = z.infer<typeof voicePageEnvelopeSchema>["data"];

export const removeVoiceEnvelopeSchema = z.object({
  success: z.literal(true),
  data: z.object({
    outcome: z.enum(["deactivated", "deleted"]),
    voice: voiceSchema,
  }),
});
export type RemoveVoiceResult = z.infer<typeof removeVoiceEnvelopeSchema>["data"];

export const voiceEnvelopeSchema = z.object({
  success: z.literal(true),
  data: voiceSchema,
});
