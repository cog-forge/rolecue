import { isAxiosError } from "axios";

// Reads the Go envelope { success:false, error:{ code, message } } without trusting its shape.
export function apiError(error: unknown) {
  if (!isAxiosError<{ error?: { code?: unknown; message?: unknown } }>(error))
    return { status: 0, code: null, message: null };
  const body = error.response?.data?.error;
  return {
    status: error.response?.status ?? 0,
    code: typeof body?.code === "string" ? body.code : null,
    message: typeof body?.message === "string" ? body.message : null,
  };
}

export function isAccessDenied(error: unknown) {
  return [401, 403].includes(apiError(error).status);
}
