import type { InternalAxiosRequestConfig } from "axios";
import { beforeEach, describe, expect, it } from "vitest";
import { apiClient } from "@/lib/api/client";
import { useAuthStore } from "@/stores/auth-store";

describe("auth store", () => {
  beforeEach(() => {
    useAuthStore.getState().clearSession();
    window.sessionStorage.clear();
  });

  it("sets the access token and persists only cross-route auth state in the tab", () => {
    useAuthStore.getState().setAccessToken("test-token");

    expect(useAuthStore.getState().accessToken).toBe("test-token");
    expect(window.sessionStorage.getItem("rolecue-auth")).toContain(
      "test-token",
    );
  });

  it("clears the token from memory and sessionStorage", () => {
    useAuthStore.getState().setAccessToken("test-token");
    useAuthStore.getState().clearSession();

    expect(useAuthStore.getState().accessToken).toBeNull();
    expect(window.sessionStorage.getItem("rolecue-auth")).toContain(
      '"accessToken":null',
    );
  });

  it("adds the current access token as a bearer header on shared API requests", async () => {
    useAuthStore.getState().setAccessToken("test-token");
    const previousBaseURL = apiClient.defaults.baseURL;
    const previousAdapter = apiClient.defaults.adapter;
    let requestConfig: InternalAxiosRequestConfig | undefined;
    apiClient.defaults.baseURL = "https://go.example.test";
    apiClient.defaults.adapter = async (config) => {
      requestConfig = config;
      return {
        data: [],
        status: 200,
        statusText: "OK",
        headers: {},
        config,
      };
    };

    try {
      await apiClient.get("/jds");
      expect(requestConfig?.headers.get("Authorization")).toBe(
        "Bearer test-token",
      );
      expect(requestConfig?.withCredentials).toBe(true);
    } finally {
      apiClient.defaults.baseURL = previousBaseURL;
      apiClient.defaults.adapter = previousAdapter;
    }
  });
});
