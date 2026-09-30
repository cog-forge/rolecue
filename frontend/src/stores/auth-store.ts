"use client";

import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";

export type AuthState = {
  accessToken: string | null;
  setAccessToken: (accessToken: string) => void;
  clearSession: () => void;
};

/** Tab-scoped access token only. Authentication derives from Boolean(accessToken). */
export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      accessToken: null,
      setAccessToken: (accessToken) => set({ accessToken }),
      clearSession: () => set({ accessToken: null }),
    }),
    {
      name: "rolecue-auth",
      storage: createJSONStorage(() => ({
        getItem: (name) => {
          try {
            return typeof window === "undefined"
              ? null
              : window.sessionStorage.getItem(name);
          } catch {
            return null;
          }
        },
        setItem: (name, value) => {
          try {
            if (typeof window !== "undefined")
              window.sessionStorage.setItem(name, value);
          } catch {}
        },
        removeItem: (name) => {
          try {
            if (typeof window !== "undefined")
              window.sessionStorage.removeItem(name);
          } catch {}
        },
      })),
      partialize: ({ accessToken }) => ({ accessToken }),
    },
  ),
);
