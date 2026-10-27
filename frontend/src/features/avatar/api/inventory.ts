import { isAxiosError } from "axios";
import { z } from "zod";
import { apiClient } from "@/lib/api/client";
import {
  avatarDetailEnvelopeSchema,
  avatarEnvelopeSchema,
  avatarMetadataSchema,
  avatarPageEnvelopeSchema,
  capacityEnvelopeSchema,
  purchaseEnvelopeSchema,
  purchaseFormSchema,
  type AvatarMetadata,
} from "../schemas/avatar-schema";

export const avatarPageSize = 12;

export const avatarKeys = {
  all: (userId: string) => ["inventory", userId] as const,
  summary: (userId: string) => [...avatarKeys.all(userId), "summary"] as const,
  lists: (userId: string) => [...avatarKeys.all(userId), "avatars"] as const,
  list: (userId: string, page: number) =>
    [...avatarKeys.lists(userId), page] as const,
  detail: (userId: string, id: string) =>
    [...avatarKeys.all(userId), "avatar", id] as const,
};

// Outside the inventory hierarchy on purpose: refreshing the inventory after a change must not
// re-render every model. A new `updated_at` is a new portrait.
export const avatarPortraitKey = (
  userId: string,
  avatarId: string,
  version: string,
) => ["avatar-portrait", userId, avatarId, version] as const;

export async function getInventory(signal?: AbortSignal) {
  const response = await apiClient.get<unknown>("/inventory", { signal });
  return capacityEnvelopeSchema.parse(response.data).data;
}

export async function listAvatars(page: number, signal?: AbortSignal) {
  const response = await apiClient.get<unknown>("/inventory/avatars", {
    params: { limit: avatarPageSize, offset: (page - 1) * avatarPageSize },
    signal,
  });
  return avatarPageEnvelopeSchema.parse(response.data).data;
}

// The presigned VRM grant expires after five minutes; callers keep it in memory only.
export async function getAvatar(id: string, signal?: AbortSignal) {
  const response = await apiClient.get<unknown>(
    `/inventory/avatars/${encodeURIComponent(id)}`,
    { signal },
  );
  return avatarDetailEnvelopeSchema.parse(response.data).data;
}

export async function deleteAvatar(id: string) {
  const response = await apiClient.delete<unknown>(
    `/inventory/avatars/${encodeURIComponent(id)}`,
  );
  if (response.status !== 204) throw new UnexpectedResponseError();
}

export async function purchaseSlots(amount: number) {
  const body = purchaseFormSchema.parse({ amount });
  const response = await apiClient.post<unknown>("/inventory/purchase", body);
  return purchaseEnvelopeSchema.parse(response.data).data;
}

export async function createAvatar(
  file: Blob,
  metadata: AvatarMetadata,
  signal: AbortSignal,
) {
  const form = new FormData();
  form.append("file", file, "avatar.glb");
  form.append("metadata", JSON.stringify(avatarMetadataSchema.parse(metadata)));
  // Conversion is synchronous and can take minutes; the API bounds it with its own deadline.
  const response = await apiClient.post<unknown>("/inventory/avatars", form, {
    signal,
    timeout: 0,
  });
  return avatarEnvelopeSchema.parse(response.data).data;
}

class UnexpectedResponseError extends Error {}

const errorEnvelopeSchema = z.object({
  success: z.literal(false),
  error: z.object({ code: z.string(), message: z.string() }),
});

// Codes that confirm the operation did not happen. Anything else (no envelope, a proxy error,
// INTERNAL_ERROR, a 2xx body that fails validation, a dropped connection) may follow a commit.
const confirmedCodes = {
  create: [
    "VALIDATION_ERROR",
    "INVENTORY_FULL",
    "AVATAR_SOURCE_TOO_LARGE",
    "AVATAR_SOURCE_UNSUPPORTED",
    "AVATAR_CONVERTER_UNAVAILABLE",
    "AVATAR_CONVERSION_FAILED",
    "AVATAR_CONVERSION_TIMEOUT",
    "AVATAR_PERSISTENCE_FAILED",
  ],
  purchase: ["VALIDATION_ERROR", "INSUFFICIENT_BALANCE"],
  delete: ["VALIDATION_ERROR", "AVATAR_NOT_FOUND", "AVATAR_IN_USE"],
} as const;

const accessCodes = ["INVALID_TOKEN", "FORBIDDEN", "USER_INACTIVE"];

export type InventoryOperation = keyof typeof confirmedCodes;
export type InventoryFailure =
  | { confirmed: true; code: string; denied: boolean }
  | { confirmed: false; denied: false };

export function inventoryFailure(
  error: unknown,
  operation: InventoryOperation,
): InventoryFailure {
  if (isAxiosError(error)) {
    const body = errorEnvelopeSchema.safeParse(error.response?.data);
    const status = error.response?.status ?? 0;
    if (body.success) {
      const { code } = body.data.error;
      const denied = [401, 403].includes(status) && accessCodes.includes(code);
      if (
        denied ||
        (confirmedCodes[operation] as readonly string[]).includes(code)
      )
        return { confirmed: true, code, denied };
    }
  }
  return { confirmed: false, denied: false };
}

export function isAccessDenied(error: unknown) {
  return (
    isAxiosError(error) && [401, 403].includes(error.response?.status ?? 0)
  );
}
