import { z } from "zod";
import { normalizeRoleText } from "../lib/role-text";
import { guardMessages, type RoleValidation } from "./role-validation";

export function createEntrySchema(v: RoleValidation) {
  return z.object({
    rawText: z.string().refine((value) => {
      const length = [...normalizeRoleText(value)].length;
      return length >= 100 && length <= 20_000;
    }, v("entryLength")),
  });
}
export const entrySchema = createEntrySchema(guardMessages);

export type EntryValues = z.infer<typeof entrySchema>;
