import { z } from "zod";

export const transactionStatusSchema = z.enum([
  "pending",
  "success",
  "failed",
  "cancelled",
]);

export type TransactionStatus = z.infer<typeof transactionStatusSchema>;

export const walletSchema = z.object({
  id: z.string().min(1),
  user_id: z.string().min(1),
  balance: z.number().int(),
});

export type Wallet = z.infer<typeof walletSchema>;

export const walletEnvelopeSchema = z.object({
  success: z.boolean(),
  data: walletSchema,
});

export const coinPackageSchema = z.object({
  id: z.string().min(1),
  name: z.string(),
  price_vnd: z.number().int(),
  description: z.string().nullable().optional(),
  coin_amount: z.number().int(),
  currency: z.string().default("VND"),
  is_active: z.boolean().default(true),
  created_at: z.string().optional(),
  updated_at: z.string().optional(),
});

export type CoinPackage = z.infer<typeof coinPackageSchema>;

export const coinPackageListEnvelopeSchema = z.object({
  success: z.boolean(),
  data: z.array(coinPackageSchema),
});

export const transactionSchema = z.object({
  id: z.string().min(1),
  from: z.string().nullable().optional(),
  to: z.string().nullable().optional(),
  amount: z.number().int(),
  currency: z.string().default("COIN"),
  description: z.string().nullable().optional(),
  status: transactionStatusSchema,
  payos_order_code: z.number().nullable().optional(),
  created_at: z.string().optional(),
  updated_at: z.string().optional(),
});

export type Transaction = z.infer<typeof transactionSchema>;

export const transactionPageSchema = z.object({
  items: z.array(transactionSchema),
  total: z.number().int(),
  page: z.number().int(),
  page_size: z.number().int(),
});

export type TransactionPage = z.infer<typeof transactionPageSchema>;

export const transactionPageEnvelopeSchema = z.object({
  success: z.boolean(),
  data: transactionPageSchema,
});

export const depositCheckoutRequestSchema = z.object({
  package_id: z.string().min(1),
  return_url: z.string().url().optional(),
  cancel_url: z.string().url().optional(),
});

export type DepositCheckoutRequest = z.infer<
  typeof depositCheckoutRequestSchema
>;

export const depositCheckoutResponseSchema = z.object({
  checkout_url: z.string(),
  order_code: z.number().int(),
  amount: z.number().int(),
  coin_amount: z.number().int(),
  transaction_id: z.string().min(1),
});

export type DepositCheckoutResponse = z.infer<
  typeof depositCheckoutResponseSchema
>;

export const depositCheckoutEnvelopeSchema = z.object({
  success: z.boolean(),
  data: depositCheckoutResponseSchema,
});

export const syncDepositRequestSchema = z.object({
  order_code: z.number().int().positive(),
});

export type SyncDepositRequest = z.infer<typeof syncDepositRequestSchema>;

export const syncDepositEnvelopeSchema = z.object({
  success: z.literal(true),
  data: transactionSchema,
});
