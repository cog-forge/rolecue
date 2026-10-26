import axios, { isAxiosError } from "axios";
import type { UploadTicket } from "./contract";

export async function uploadPDF(
  ticket: UploadTicket,
  file: File,
  signal: AbortSignal,
  progress: (percent: number) => void,
) {
  if (ticket.request.Method !== "PUT") throw new Error("Invalid upload method");
  // Isolated storage transport: never attach the API client's cookies/interceptors.
  await axios.request({
    url: ticket.request.URL,
    method: "PUT",
    headers: ticket.request.Headers,
    data: file,
    withCredentials: false,
    signal,
    timeout: 120_000,
    onUploadProgress: (event) =>
      progress(Math.min(100, Math.round((event.loaded / file.size) * 100))),
  });
}
export function roleError(error: unknown) {
  if (isAxiosError(error)) {
    if (error.code === "ERR_CANCELED")
      return "Canceled. You can retry or open your saved draft.";
    const payload: unknown = error.response?.data;
    if (payload && typeof payload === "object" && "error" in payload) {
      const detail = payload.error;
      if (
        detail &&
        typeof detail === "object" &&
        "message" in detail &&
        typeof detail.message === "string"
      )
        return detail.message;
    }
    if (error.response?.status === 409)
      return "This job description is not ready for that action. Refresh and try again.";
  }
  return "We couldn't complete that step. Please try again.";
}
