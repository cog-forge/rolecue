import { z } from "zod";
import { isAxiosError } from "axios";
import { apiClient } from "@/lib/api/client";
import { banRequestSchema, targetRequestSchema } from "../schemas/account-lock";
import { apiErrorMessage, type ErrorTranslator } from "@/lib/i18n/api-errors";

export const ACCOUNTS_PAGE_SIZE = 20;

const accountSchema = z
  .object({
    id: z.uuid(),
    name: z.string(),
    email: z.email(),
    image: z.string().nullish(),
    role: z.string(),
    is_locked: z.boolean(),
    lock_reason: z.string().nullish(),
    lock_expires_at: z.coerce.date().nullish(),
    created_at: z.coerce.date(),
    updated_at: z.coerce.date().nullish(),
    email_verified: z.boolean(),
  })
  .transform((account) => ({
    id: account.id,
    name: account.name,
    email: account.email,
    image: account.image,
    role: account.role,
    banned: account.is_locked,
    banReason: account.lock_reason,
    banExpires: account.lock_expires_at,
    createdAt: account.created_at,
    updatedAt: account.updated_at,
    emailVerified: account.email_verified,
  }));
const listSchema = z.object({
  users: z.array(accountSchema),
  total: z.number().int().nonnegative(),
});
export type AdminAccount = z.infer<typeof accountSchema>;
export type AccountList = z.infer<typeof listSchema>;
export const accountFilters = z.enum([
  "all",
  "locked",
  "candidate",
  "recruiter",
  "admin",
]);
export type AccountQuery = {
  offset: number;
  searchValue: string;
  filter: z.infer<typeof accountFilters>;
};
export const accountsKeys = {
  viewer: (viewerId: string) => ["auth", "admin-users", viewerId] as const,
  lists: (viewerId: string) =>
    [...accountsKeys.viewer(viewerId), "list"] as const,
  list: (viewerId: string, query: AccountQuery) =>
    [...accountsKeys.lists(viewerId), query] as const,
  detail: (viewerId: string, id: string) =>
    [...accountsKeys.viewer(viewerId), "detail", id] as const,
};

export function isAdminAccessError(error: unknown) {
  return (
    isAxiosError(error) && [401, 403].includes(error.response?.status ?? 0)
  );
}

export async function listAccounts(query: AccountQuery, signal?: AbortSignal) {
  const response = await apiClient.get<unknown>("/admin/users", {
    params: {
      limit: ACCOUNTS_PAGE_SIZE,
      offset: query.offset,
      search: query.searchValue,
      filter: query.filter,
    },
    signal,
  });
  return z
    .object({ success: z.literal(true), data: listSchema })
    .parse(response.data).data;
}

export async function getAccount(id: string, signal?: AbortSignal) {
  const response = await apiClient.get<unknown>(
    `/admin/users/${z.uuid().parse(id)}`,
    { signal },
  );
  return z
    .object({ success: z.literal(true), data: accountSchema })
    .parse(response.data).data;
}

export type AccountAction =
  | { kind: "lock"; request: z.infer<typeof banRequestSchema> }
  | { kind: "unlock"; request: z.infer<typeof targetRequestSchema> };

export async function updateAccountLock(action: AccountAction) {
  const { userId } = targetRequestSchema.parse(action.request);
  let body: { reason?: string; expires_in?: number } = {};
  if (action.kind === "lock") {
    const request = banRequestSchema.parse(action.request);
    body = { reason: request.banReason, expires_in: request.banExpiresIn };
  }
  const response = await apiClient.post<unknown>(
    `/admin/users/${userId}/${action.kind}`,
    body,
  );
  return z
    .object({
      success: z.literal(true),
      data: z.object({
        user: accountSchema,
        changed: z.boolean(),
        notification: z.object({
          status: z.enum(["accepted", "failed", "not-needed"]),
        }),
      }),
    })
    .parse(response.data).data;
}

// A client error is shown by its stable API code in the active language; the server's own text
// never reaches the screen.
export function getAccountActionError(error: unknown, t: ErrorTranslator) {
  const fallback =
    "We couldn't confirm the change. Please refresh before trying again.";
  return isAxiosError(error) && error.response && error.response.status < 500
    ? apiErrorMessage(t, error, fallback)
    : fallback;
}
