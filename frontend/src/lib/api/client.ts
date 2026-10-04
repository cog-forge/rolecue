"use client";

import axios from "axios";
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

export { apiClient };
