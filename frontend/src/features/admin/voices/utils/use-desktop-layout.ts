import { useSyncExternalStore } from "react";

// Matches Tailwind's lg breakpoint: from here the voice details sit beside the grid instead of
// opening in a sheet.
const query = "(min-width: 1024px)";

function subscribe(onChange: () => void) {
  if (typeof window.matchMedia !== "function") return () => {};
  const media = window.matchMedia(query);
  media.addEventListener("change", onChange);
  return () => media.removeEventListener("change", onChange);
}

const snapshot = () =>
  typeof window.matchMedia === "function" && window.matchMedia(query).matches;

export function useDesktopLayout() {
  return useSyncExternalStore(subscribe, snapshot, () => false);
}
