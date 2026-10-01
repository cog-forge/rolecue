"use client";

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";
import { Moon, Sun } from "lucide-react";
import { Button } from "@/components/ui/button";

type Theme = "light" | "dark";
type CurtainPhase = "idle" | "falling" | "rising";

type CurtainThemeToggleProps = {
  duration?: number;
  onThemeChange?: (theme: Theme) => void;
};

const themeStorageKey = "rolecue-theme";
const themeChangeEvent = "rolecue-theme-change";

function subscribeToTheme(callback: () => void) {
  window.addEventListener(themeChangeEvent, callback);
  return () => window.removeEventListener(themeChangeEvent, callback);
}

function getCurrentTheme(): Theme {
  return document.documentElement.classList.contains("dark") ? "dark" : "light";
}

function getServerTheme(): Theme {
  return "light";
}

export function CurtainThemeToggle({
  duration = 550,
  onThemeChange,
}: CurtainThemeToggleProps) {
  const theme = useSyncExternalStore(
    subscribeToTheme,
    getCurrentTheme,
    getServerTheme,
  );
  const [phase, setPhase] = useState<CurtainPhase>("idle");
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (timeoutRef.current) clearTimeout(timeoutRef.current);
    };
  }, []);

  const applyTheme = useCallback(
    (nextTheme: Theme) => {
      document.documentElement.classList.toggle("dark", nextTheme === "dark");
      try {
        window.localStorage.setItem(themeStorageKey, nextTheme);
      } catch {
        // The theme still works when browser storage is unavailable.
      }
      window.dispatchEvent(new Event(themeChangeEvent));
      onThemeChange?.(nextTheme);
    },
    [onThemeChange],
  );

  const toggleTheme = useCallback(() => {
    if (phase !== "idle") return;

    const nextTheme = theme === "light" ? "dark" : "light";
    const reduceMotion = window.matchMedia(
      "(prefers-reduced-motion: reduce)",
    ).matches;

    if (reduceMotion || duration <= 0) {
      applyTheme(nextTheme);
      return;
    }

    setPhase("falling");
    timeoutRef.current = setTimeout(() => {
      applyTheme(nextTheme);
      setPhase("rising");
      timeoutRef.current = setTimeout(() => {
        setPhase("idle");
        timeoutRef.current = null;
      }, duration);
    }, duration);
  }, [applyTheme, duration, phase, theme]);

  return (
    <>
      <div
        aria-hidden="true"
        className="pointer-events-none fixed inset-0 z-[9997] origin-top"
        style={{
          background: "var(--rolecue-canvas)",
          transform: phase === "falling" ? "scaleY(1)" : "scaleY(0)",
          transition:
            phase === "idle"
              ? "none"
              : `transform ${duration}ms cubic-bezier(0.76, 0, 0.24, 1)`,
        }}
      />
      <Button
        aria-label={
          theme === "light" ? "Switch to dark theme" : "Switch to light theme"
        }
        aria-pressed={theme === "dark"}
        className="relative size-11 shrink-0 rounded-full border-(--rolecue-border) bg-(--rolecue-surface) p-0 text-(--rolecue-ink) shadow-none transition-transform duration-(--duration-quick) hover:scale-[1.04] hover:bg-(--rolecue-surface-soft) hover:text-(--rolecue-ink) active:scale-[0.96] motion-reduce:transform-none! motion-reduce:transition-none!"
        onClick={toggleTheme}
        size="icon"
        type="button"
        variant="outline"
      >
        {theme === "light" ? (
          <Moon aria-hidden="true" size={17} strokeWidth={1.8} />
        ) : (
          <Sun aria-hidden="true" size={17} strokeWidth={1.8} />
        )}
      </Button>
    </>
  );
}
