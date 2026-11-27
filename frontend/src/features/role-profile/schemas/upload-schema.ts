import { z } from "zod";
import { signedStorageRequestSchema } from "@/lib/api/signed-storage-request";

export const maxPDFBytes = 5 * 1024 * 1024;
export const uploadTicketSchema = z
  .object({
    uploadId: z.uuid(),
    request: signedStorageRequestSchema,
  })
  .strict();
export const uploadEnvelopeSchema = z.object({
  success: z.literal(true),
  data: uploadTicketSchema,
});
export const documentEnvelopeSchema = z.object({
  success: z.literal(true),
  data: signedStorageRequestSchema,
});

export type UploadTicket = z.infer<typeof uploadTicketSchema>;
