"use client";

import {
  Component,
  useCallback,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
  type ReactNode,
} from "react";
import dynamic from "next/dynamic";
import { Box, LoaderCircle, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { CanvasStatus } from "./avatar-canvas";
import { StudioBackdrop } from "./studio-backdrop";
import { isWebGLAvailable } from "./webgl";

/** A temporary, private GET grant for one VRM file (e.g. `model_access` from the inventory API). */
export type ModelAccess = {
  url: string;
  method: "GET";
  headers: Record<string, string>;
  expiresAt: string;
};

const AvatarCanvas = dynamic(() => import("./avatar-canvas"), {
  ssr: false,
  loading: () => null,
});

const noSubscription = () => () => {};

const messages = {
  expired: "The preview link expired. Try again to get a fresh one.",
  network:
    "We couldn't download this avatar. Check your connection and try again.",
  invalid: "This avatar file couldn't be displayed.",
} as const;

/**
 * Reusable 3D VRM viewer: idle pose, orbit/zoom and reset, with a 2D fallback when WebGL or the model
 * fails. The owner supplies the grant and re-fetches it when `onAccessExpired` is called; one expiry is
 * recovered automatically, a second one in a row shows the error so it can't loop.
 */
export function AvatarViewer({
  access,
  label,
  onAccessExpired,
  className,
}: {
  access: ModelAccess | null;
  label: string;
  onAccessExpired: () => void;
  className?: string;
}) {
  const webgl = useSyncExternalStore(
    noSubscription,
    isWebGLAvailable,
    () => null,
  );
  const [status, setStatus] = useState<CanvasStatus>({ state: "loading" });
  const [attempt, setAttempt] = useState(0);
  const [resetSignal, setResetSignal] = useState(0);
  const expiredOnce = useRef(false);
  const requestAccess = useRef(onAccessExpired);
  useEffect(() => {
    requestAccess.current = onAccessExpired;
  }, [onAccessExpired]);

  const handleStatus = useCallback((next: CanvasStatus) => {
    if (
      next.state === "error" &&
      next.kind === "expired" &&
      !expiredOnce.current
    ) {
      expiredOnce.current = true;
      setStatus({ state: "loading" });
      requestAccess.current();
      return;
    }
    if (next.state === "ready") expiredOnce.current = false;
    setStatus(next);
  }, []);

  const retry = () => {
    expiredOnce.current = false;
    setStatus({ state: "loading" });
    if (status.state === "error" && status.kind === "expired")
      requestAccess.current();
    else setAttempt((value) => value + 1);
  };

  const fallback = (reason: string, canRetry: boolean) => (
    <FallbackCard
      label={label}
      reason={reason}
      onRetry={canRetry ? retry : undefined}
    />
  );

  return (
    <div
      className={cn(
        "relative isolate aspect-[3/4] w-full overflow-hidden rounded-2xl border sm:aspect-[4/3]",
        className,
      )}
    >
      <StudioBackdrop />
      {webgl === false ? (
        fallback("3D preview isn't available on this device or browser.", false)
      ) : status.state === "fallback" ? (
        fallback("3D rendering stopped on this device.", true)
      ) : status.state === "error" ? (
        fallback(messages[status.kind], true)
      ) : (
        <>
          {webgl && access && (
            <ViewerBoundary
              key={attempt}
              fallback={fallback(
                "3D preview couldn't start on this device.",
                true,
              )}
            >
              <div role="img" aria-label={label} className="absolute inset-0">
                <AvatarCanvas
                  access={access}
                  resetSignal={resetSignal}
                  onStatus={handleStatus}
                />
              </div>
            </ViewerBoundary>
          )}
          {status.state === "loading" && (
            <div
              role="status"
              className="absolute inset-0 flex flex-col items-center justify-center gap-3 bg-background/90 text-sm text-muted-foreground"
            >
              <LoaderCircle
                aria-hidden
                className="size-5 animate-spin motion-reduce:animate-none"
              />
              Loading 3D preview…
            </div>
          )}
          {status.state === "ready" && (
            <Button
              type="button"
              variant="secondary"
              className="absolute right-3 bottom-3 h-11"
              onClick={() => setResetSignal((value) => value + 1)}
            >
              <RotateCcw aria-hidden /> Reset view
            </Button>
          )}
        </>
      )}
    </div>
  );
}

function FallbackCard({
  label,
  reason,
  onRetry,
}: {
  label: string;
  reason: string;
  onRetry?: () => void;
}) {
  return (
    <div
      role="alert"
      className="absolute inset-0 flex flex-col items-center justify-center gap-4 bg-background/90 p-6 text-center"
    >
      <span
        aria-hidden
        className="flex size-14 items-center justify-center rounded-2xl border bg-card"
      >
        <Box className="size-6 text-muted-foreground" strokeWidth={1.5} />
      </span>
      <div className="space-y-1">
        <p className="text-sm font-medium [overflow-wrap:anywhere]">{label}</p>
        <p className="max-w-xs text-sm text-muted-foreground">{reason}</p>
      </div>
      {onRetry && (
        <Button
          type="button"
          variant="outline"
          className="h-11"
          onClick={onRetry}
        >
          Try again
        </Button>
      )}
    </div>
  );
}

// Render or chunk-loading failures inside the 3D tree show the 2D card instead of breaking the page.
class ViewerBoundary extends Component<
  { children: ReactNode; fallback: ReactNode },
  { failed: boolean }
> {
  state = { failed: false };
  static getDerivedStateFromError() {
    return { failed: true };
  }
  render() {
    return this.state.failed ? this.props.fallback : this.props.children;
  }
}
