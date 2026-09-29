"use client";

import axios from "axios";
import { getApiBaseUrl } from "./config";
import { useAuthStore } from "@/stores/auth-store";

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
  const token = useAuthStore.getState().accessToken;
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

export { apiClient };
