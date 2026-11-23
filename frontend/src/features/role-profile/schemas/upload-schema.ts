import { z } from "zod";

export const maxPDFBytes = 5 * 1024 * 1024;
export const signedRequestSchema = z
  .object({
    URL: z
      .url()
      .refine((value) => ["http:", "https:"].includes(new URL(value).protocol)),
    Method: z.enum(["PUT", "GET"]),
    Headers: z.record(z.string(), z.string()),
    ExpiresAt: z.string(),
  })
  .strict();
export const uploadTicketSchema = z
  .object({
    uploadId: z.uuid(),
    request: signedRequestSchema,
  })
  .strict();
export const uploadEnvelopeSchema = z.object({
  success: z.literal(true),
  data: uploadTicketSchema,
});
export const documentEnvelopeSchema = z.object({
  success: z.literal(true),
  data: signedRequestSchema,
});

export type UploadTicket = z.infer<typeof uploadTicketSchema>;
