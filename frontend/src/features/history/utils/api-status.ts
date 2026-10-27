import { isAxiosError } from "axios";

// Reads the HTTP status of a failed Go call without trusting the error body.
export function apiStatus(error: unknown) {
  return isAxiosError(error) ? (error.response?.status ?? 0) : 0;
}

export function isAccessDenied(error: unknown) {
  return [401, 403].includes(apiStatus(error));
}
