import { createStore } from "zustand/vanilla";
import { createJSONStorage, persist } from "zustand/middleware";

export const workspacePreferenceKey = "rolecue.workspace-preferences";
export type Theme = "light" | "dark" | "system";
export type Preferences = { theme: Theme; collapsed: boolean | null };

export function parseWorkspacePreferences(value: unknown): Preferences {
  if (typeof value === "string") {
    try {
      value = JSON.parse(value);
    } catch {
      value = null;
    }
  }
  if (value && typeof value === "object" && "state" in value) {
    value = value.state;
  }
  if (value && typeof value === "object") {
    const stored = value as Record<string, unknown>;
    return {
      theme:
        stored.theme === "light" ||
        stored.theme === "dark" ||
        stored.theme === "system"
          ? stored.theme
          : "system",
      collapsed:
        typeof stored.collapsed === "boolean" ? stored.collapsed : null,
    };
  }
  return { theme: "system", collapsed: null };
}

export type WorkspacePreferencesState = Preferences & {
  setTheme: (theme: Theme) => void;
  setCollapsed: (collapsed: boolean) => void;
};

export function createWorkspacePreferencesStore() {
  return createStore<WorkspacePreferencesState>()(
    persist(
      (set) => ({
        theme: "system",
        collapsed: null,
        setTheme: (theme) => set({ theme }),
        setCollapsed: (collapsed) => set({ collapsed }),
      }),
      {
        name: workspacePreferenceKey,
        storage: createJSONStorage(() => window.localStorage, {
          reviver: (key, value) => {
            if (
              key === "" &&
              value &&
              typeof value === "object" &&
              !("state" in value)
            ) {
              return { state: value, version: 0 };
            }
            return value;
          },
        }),
        skipHydration: true,
        partialize: ({ theme, collapsed }) => ({ theme, collapsed }),
        merge: (persisted, current) => ({
          ...current,
          ...parseWorkspacePreferences(persisted),
        }),
      },
    ),
  );
}
