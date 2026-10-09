"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";
import { flushSync } from "react-dom";
import { usePathname } from "next/navigation";
import { useStore } from "zustand";
import { isWorkspacePath } from "@/config/navigation";
import {
  createWorkspacePreferencesStore,
  workspacePreferenceKey,
  type Theme,
} from "@/lib/stores/workspace-preferences";

export { parseWorkspacePreferences } from "@/lib/stores/workspace-preferences";
export type { Theme } from "@/lib/stores/workspace-preferences";
function useMedia(query: string) {
  const subscribeMedia = useCallback(
    (listener: () => void) => {
      const media = window.matchMedia(query);
      media.addEventListener("change", listener);
      return () => media.removeEventListener("change", listener);
    },
    [query],
  );
  const getSnapshot = useCallback(
    () => window.matchMedia(query).matches,
    [query],
  );
  return useSyncExternalStore(subscribeMedia, getSnapshot, () => null);
}
type WorkspacePreferences = {
  ready: boolean;
  mobile: boolean;
  reducedMotion: boolean;
  collapsed: boolean;
  theme: Theme;
  toggleCollapsed: () => void;
  setTheme: (theme: Theme, origin: HTMLElement) => Promise<void>;
};
const PreferencesContext = createContext<WorkspacePreferences | null>(null);

export function WorkspacePreferencesProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const [store] = useState(createWorkspacePreferencesStore);
  const [hydrated, setHydrated] = useState(false);
  const theme = useStore(store, (state) => state.theme);
  const collapsedPreference = useStore(store, (state) => state.collapsed);
  const desktop = useMedia("(min-width: 1024px)");
  const mobile = useMedia("(max-width: 767px)");
  const reducedMotion = useMedia("(prefers-reduced-motion: reduce)");
  const systemDark = useMedia("(prefers-color-scheme: dark)");
  const dark = theme === "dark" || (theme === "system" && systemDark === true);
  const collapsed = collapsedPreference ?? !desktop;
  const pathname = usePathname();
  const transitionBusy = useRef(false);
  useEffect(() => {
    let active = true;
    void Promise.resolve(store.persist.rehydrate()).finally(() => {
      if (active) setHydrated(true);
    });
    const onStorage = (event: StorageEvent) => {
      if (!event.key || event.key === workspacePreferenceKey) {
        void store.persist.rehydrate();
      }
    };
    window.addEventListener("storage", onStorage);
    return () => {
      active = false;
      window.removeEventListener("storage", onStorage);
    };
  }, [store]);

  useEffect(() => {
    if (!hydrated || systemDark === null) return;
    document.documentElement.classList.toggle(
      "dark",
      isWorkspacePath(pathname) && dark,
    );
  }, [pathname, dark, hydrated, systemDark]);

  const setTheme = async (next: Theme, origin: HTMLElement) => {
    if (transitionBusy.current) return;
    const nextDark =
      next === "dark" || (next === "system" && systemDark === true);
    const apply = () => {
      flushSync(() => {
        try {
          store.getState().setTheme(next);
        } catch {
          // Keep the in-memory preference if browser storage is unavailable.
        }
      });
      document.documentElement.classList.toggle(
        "dark",
        isWorkspacePath(pathname) && nextDark,
      );
    };
    if (
      nextDark === dark ||
      !document.startViewTransition ||
      window.matchMedia("(prefers-reduced-motion: reduce)").matches
    ) {
      apply();
      return;
    }
    transitionBusy.current = true;
    const bounds = origin.getBoundingClientRect();
    const x = bounds.left + bounds.width / 2;
    const y = bounds.top + bounds.height / 2;
    const radius = Math.hypot(
      Math.max(x, innerWidth - x),
      Math.max(y, innerHeight - y),
    );
    document.documentElement.dataset.themeReveal = "true";
    let transition: ViewTransition | undefined;
    try {
      transition = document.startViewTransition(apply);
      await transition.ready;
      await document.documentElement.animate(
        {
          clipPath: [
            `circle(0px at ${x}px ${y}px)`,
            `circle(${radius}px at ${x}px ${y}px)`,
          ],
        },
        {
          duration: 700,
          easing: "cubic-bezier(.32,.72,0,1)",
          pseudoElement: "::view-transition-new(root)",
        },
      ).finished;
      await transition.finished;
    } catch {
      // Unsupported/aborted transitions must still leave the selected theme applied.
      if (transition) transition.skipTransition();
      else apply();
    } finally {
      delete document.documentElement.dataset.themeReveal;
      transitionBusy.current = false;
    }
  };
  return (
    <PreferencesContext.Provider
      value={{
        ready:
          hydrated &&
          desktop !== null &&
          mobile !== null &&
          systemDark !== null &&
          reducedMotion !== null,
        mobile: mobile ?? false,
        reducedMotion: reducedMotion ?? true,
        collapsed,
        theme,
        toggleCollapsed: () => {
          try {
            store.getState().setCollapsed(!collapsed);
          } catch {
            // Keep the in-memory preference if browser storage is unavailable.
          }
        },
        setTheme,
      }}
    >
      {children}
    </PreferencesContext.Provider>
  );
}
export function useWorkspacePreferences() {
  const value = useContext(PreferencesContext);
  if (!value)
    throw new Error(
      "Workspace preferences require WorkspacePreferencesProvider",
    );
  return value;
}
