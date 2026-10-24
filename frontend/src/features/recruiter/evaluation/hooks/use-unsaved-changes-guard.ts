"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";

// Marks the extra history entry that keeps a Back press on the page.
const GUARD_KEY = "__rolecueUnsavedChanges";

type Leaving = { kind: "link"; href: string } | { kind: "history" };

/**
 * While `active`, asks before the page is left with unsaved changes: closing or reloading the tab
 * (the browser's own prompt), clicking an in-app link anywhere on the page, and the browser's
 * Back/Forward buttons. The App Router has no navigation events, so links are caught in the click
 * capture phase and Back is caught with an extra entry for the same URL that a Back press only
 * pops. Render a confirmation while `leaving` is set and answer it with `stay` or `leave`.
 */
export function useUnsavedChangesGuard(active: boolean) {
  const router = useRouter();
  const [leaving, setLeaving] = useState<Leaving | null>(null);
  // The guarded page's URL and history state, to put the extra entry back after "Stay".
  const page = useRef<{ href: string; state: unknown }>({
    href: "",
    state: null,
  });
  // Set once the Recruiter chose to leave, so the navigation that follows is not asked about.
  const released = useRef(false);

  useEffect(() => {
    if (!active) return;
    released.current = false;
    page.current = { href: window.location.href, state: window.history.state };
    pushGuardEntry(page.current);

    const warn = (event: BeforeUnloadEvent) => {
      if (!released.current) event.preventDefault();
    };
    const click = (event: MouseEvent) => {
      if (released.current) return;
      const href = inAppHref(event);
      if (href === null) return;
      // Before React sees the click, so a Next.js Link does not navigate.
      event.preventDefault();
      event.stopPropagation();
      setLeaving({ kind: "link", href });
    };
    const popstate = (event: PopStateEvent) => {
      if (released.current) return;
      // A capture listener on window runs before the App Router's, which would render the
      // previous page. The browser already left the extra entry; "Stay" pushes it again.
      event.stopImmediatePropagation();
      setLeaving({ kind: "history" });
    };
    window.addEventListener("beforeunload", warn);
    document.addEventListener("click", click, true);
    window.addEventListener("popstate", popstate, true);
    return () => {
      window.removeEventListener("beforeunload", warn);
      document.removeEventListener("click", click, true);
      window.removeEventListener("popstate", popstate, true);
      setLeaving(null);
      // Saved or discarded: drop the extra entry without the App Router noticing, so the next
      // Back press leaves the page at once.
      if (isGuardEntry(window.history.state)) {
        const swallow = (event: PopStateEvent) =>
          event.stopImmediatePropagation();
        window.addEventListener("popstate", swallow, {
          capture: true,
          once: true,
        });
        window.history.back();
      }
    };
  }, [active]);

  const stay = useCallback(() => {
    if (leaving?.kind === "history") pushGuardEntry(page.current);
    setLeaving(null);
  }, [leaving]);

  const leave = useCallback(() => {
    if (!leaving) return;
    released.current = true;
    setLeaving(null);
    if (leaving.kind === "link") {
      // Replacing the extra entry keeps the editor in history only once.
      if (isGuardEntry(window.history.state)) router.replace(leaving.href);
      else router.push(leaving.href);
    } else if (window.location.href === page.current.href) {
      // Back popped only the extra entry: go one further, to where the Recruiter was heading.
      window.history.back();
    } else {
      // A jump of several entries already moved the URL; let the App Router render it.
      window.dispatchEvent(
        new PopStateEvent("popstate", { state: window.history.state }),
      );
    }
  }, [leaving, router]);

  return { leaving: leaving !== null, stay, leave };
}

function pushGuardEntry({ href, state }: { href: string; state: unknown }) {
  window.history.pushState(
    { ...(state as object | null), [GUARD_KEY]: true },
    "",
    href,
  );
}

function isGuardEntry(state: unknown) {
  return (
    typeof state === "object" &&
    state !== null &&
    (state as Record<string, unknown>)[GUARD_KEY] === true
  );
}

// The app path a plain left click would open, or null when the click does not leave the page
// inside the app (new tab, download, another site, or only a hash change).
function inAppHref(event: MouseEvent) {
  if (
    event.defaultPrevented ||
    event.button !== 0 ||
    event.metaKey ||
    event.ctrlKey ||
    event.shiftKey ||
    event.altKey
  )
    return null;
  const anchor =
    event.target instanceof Element ? event.target.closest("a[href]") : null;
  if (!(anchor instanceof HTMLAnchorElement)) return null;
  if (
    (anchor.target && anchor.target !== "_self") ||
    anchor.hasAttribute("download")
  )
    return null;
  const url = new URL(anchor.href, window.location.href);
  const here = window.location;
  // Another site is covered by the beforeunload prompt.
  if (url.origin !== here.origin) return null;
  if (url.pathname === here.pathname && url.search === here.search) return null;
  return `${url.pathname}${url.search}${url.hash}`;
}
