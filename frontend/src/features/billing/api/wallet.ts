import { apiClient } from "@/lib/api/client";
import {
  coinPackageListEnvelopeSchema,
  depositCheckoutEnvelopeSchema,
  syncDepositEnvelopeSchema,
  transactionPageEnvelopeSchema,
  walletEnvelopeSchema,
  type DepositCheckoutRequest,
} from "../schemas/wallet-schema";

export const walletKeys = {
  all: (userId: string) => ["wallet", userId] as const,
  wallet: (userId: string) => [...walletKeys.all(userId), "detail"] as const,
  packages: () => ["wallet", "packages"] as const,
  transactions: (userId: string, page: number, pageSize: number) =>
    [...walletKeys.all(userId), "transactions", page, pageSize] as const,
};

export async function getWallet(signal?: AbortSignal) {
  const response = await apiClient.get<unknown>("/wallet", { signal });
  return walletEnvelopeSchema.parse(response.data).data;
}

export async function getCoinPackages(signal?: AbortSignal) {
  const response = await apiClient.get<unknown>("/wallet/packages", { signal });
  return coinPackageListEnvelopeSchema.parse(response.data).data;
}

export async function getTransactions(
  page = 1,
  pageSize = 20,
  signal?: AbortSignal,
) {
  const response = await apiClient.get<unknown>("/wallet/transactions", {
    params: { page, page_size: pageSize },
    signal,
  });
  return transactionPageEnvelopeSchema.parse(response.data).data;
}

export async function createDepositCheckout(body: DepositCheckoutRequest) {
  const response = await apiClient.post<unknown>(
    "/wallet/deposit/checkout",
    body,
  );
  return depositCheckoutEnvelopeSchema.parse(response.data).data;
}

export async function syncDepositStatus(orderCode: number) {
  const response = await apiClient.post<unknown>("/wallet/deposit/sync", {
    order_code: orderCode,
  });
  return syncDepositEnvelopeSchema.parse(response.data).data;
}
