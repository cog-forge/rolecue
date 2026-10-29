import { z } from "zod";

// The dashboard only needs how many rows match, so it reads the page total and ignores the items.
export const countPageSchema = z.object({
  success: z.literal(true),
  data: z.object({
    items: z.array(z.unknown()),
    pagination: z.object({ total: z.number().int().nonnegative() }),
  }),
});

// Eligible Applications carry the interview deadline the dashboard counts down to.
export const deadlinePageSchema = z.object({
  success: z.literal(true),
  data: z.object({
    items: z.array(
      z.object({
        interview_eligible: z.boolean(),
        interview_deadline: z.iso.datetime({ offset: true }).nullable(),
      }),
    ),
    pagination: z.object({ total: z.number().int().nonnegative() }),
  }),
});

// One dashboard row: how many things need attention. atLeast means the count is a lower bound,
// because only part of the data could be inspected.
export type Todo = {
  count: number;
  atLeast: boolean;
  detail: string | null;
};
