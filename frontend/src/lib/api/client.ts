"use client";

import axios from "axios";
import {
  adminMfaSetupHref,
  isAdminMfaRequired,
} from "@/features/auth/utils/admin-mfa";
import { getApiBaseUrl } from "./config";

const apiClient = axios.create({
  baseURL: getApiBaseUrl(),
  withCredentials: true,
});

apiClient.interceptors.request.use((config) => {
  if (!config.baseURL) {
    return Promise.reject(
      new Error("NEXT_PUBLIC_API_BASE_URL is not configured."),
    );
  }
  return config;
});

// An Admin without two-factor is sent to authenticator setup instead of seeing a failed page.
apiClient.interceptors.response.use(undefined, (error: unknown) => {
  if (typeof window !== "undefined" && isAdminMfaRequired(error)) {
    const href = adminMfaSetupHref(window.location.pathname);
    if (href) window.location.assign(href);
  }
  return Promise.reject(error);
});

export { apiClient };
