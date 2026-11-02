"use client";

import { useEffect, useRef, useState } from "react";
import type { AvaturnSDK } from "@avaturn/sdk";
import { useTranslations } from "next-intl";
import {
  Camera,
  LoaderCircle,
  Maximize2,
  Minimize2,
  RotateCcw,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";

export function AvaturnCreator({
  url,
  onExport,
  expanded,
  onToggleExpanded,
}: {
  url: string;
  onExport: (payload: unknown) => void;
  expanded: boolean;
  onToggleExpanded: () => void;
}) {
  const t = useTranslations("candidate.avatarStudio.creator");
  const container = useRef<HTMLDivElement>(null);
  const frameTitle = t("frameTitle");
  // The SDK effect below must not re-run when the language changes, so it reads the title from a ref.
  const frameTitleRef = useRef(frameTitle);
  useEffect(() => {
    frameTitleRef.current = frameTitle;
    const iframe = container.current?.querySelector("iframe");
    if (iframe) iframe.title = frameTitle;
  }, [frameTitle]);
  const [attempt, setAttempt] = useState(0);
  const [confirmReload, setConfirmReload] = useState(false);
  const [status, setStatus] = useState<"loading" | "ready" | "error">(
    "loading",
  );
  function reload() {
    setConfirmReload(false);
    setStatus("loading");
    setAttempt((value) => value + 1);
  }

  useEffect(() => {
    const element = container.current;
    if (!element) return;
    let active = true;
    let sdk: AvaturnSDK | undefined;
    // SDK 1.1.4 checks the message's source label, but does not check the
    // browser origin/window. Filter its protocol messages before its listener.
    const guard = (event: MessageEvent<unknown>) => {
      const data = event.data;
      if (
        typeof data !== "object" ||
        data === null ||
        !("source" in data) ||
        data.source !== "v1.avaturn-sdk-server"
      )
        return;
      const iframe = element.querySelector("iframe");
      if (
        event.origin !== new URL(url).origin ||
        event.source !== iframe?.contentWindow
      ) {
        event.stopImmediatePropagation();
      }
    };
    window.addEventListener("message", guard, true);
    void import("@avaturn/sdk")
      .then(async ({ AvaturnSDK }) => {
        if (!active) return;
        sdk = new AvaturnSDK();
        sdk.on("export", (payload) => {
          if (active) onExport(payload);
        });
        await sdk.init(element, { url });
        if (active) setStatus("ready");
      })
      .catch(() => {
        if (active) setStatus("error");
      });
    // The SDK appends its iframe with allow="camera *; …" already set; changing allow after
    // insertion would not affect the loaded document, so only the accessible name is added.
    const observer = new MutationObserver(() => {
      const iframe = element.querySelector("iframe");
      if (iframe) iframe.title = frameTitleRef.current;
    });
    observer.observe(element, { childList: true });
    return () => {
      active = false;
      sdk?.destroy();
      window.removeEventListener("message", guard, true);
      observer.disconnect();
      // SDK.destroy removes listeners, but does not remove the iframe/camera.
      element.replaceChildren();
    };
  }, [url, onExport, attempt]);

  return (
    <div className="overflow-hidden rounded-2xl border bg-card">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-3">
        <p
          role={status === "error" ? "alert" : "status"}
          className="max-w-lg text-xs leading-relaxed text-muted-foreground sm:text-sm"
        >
          {status === "loading"
            ? t("connecting")
            : status === "error"
              ? t("openFailed")
              : t("ready")}
        </p>
        <div className="flex flex-wrap gap-2">
          <Button
            className="min-h-11 px-3"
            type="button"
            variant="ghost"
            aria-pressed={expanded}
            onClick={onToggleExpanded}
          >
            {expanded ? <Minimize2 aria-hidden /> : <Maximize2 aria-hidden />}
            {expanded ? t("restore") : t("expand")}
          </Button>
          <Button
            type="button"
            className="min-h-11 px-3"
            variant="ghost"
            onClick={() =>
              status === "error" ? reload() : setConfirmReload(true)
            }
          >
            <RotateCcw aria-hidden /> {t("reload")}
          </Button>
        </div>
      </div>
      <div className="relative">
        <div
          ref={container}
          inert={status !== "ready"}
          className={cn(
            "min-h-[32rem] w-full bg-muted/20",
            expanded ? "h-[80dvh]" : "h-[70dvh]",
          )}
        />
        <div
          aria-hidden
          className={cn(
            "absolute inset-0 flex flex-col items-center justify-center gap-4 bg-card transition-opacity duration-(--duration-slow) motion-reduce:transition-none",
            status === "loading"
              ? "opacity-100"
              : "pointer-events-none opacity-0",
          )}
        >
          <span className="flex size-16 items-center justify-center rounded-2xl border bg-muted">
            <Camera
              aria-hidden
              className="size-7 text-muted-foreground"
              strokeWidth={1.5}
            />
          </span>
          <p
            className="t-shimmer text-sm font-medium"
            data-text={t("gettingReady")}
          >
            {t("gettingReady")}
          </p>
          <LoaderCircle
            aria-hidden
            className="size-4 animate-spin text-muted-foreground motion-reduce:animate-none"
          />
        </div>
      </div>
      <Dialog open={confirmReload} onOpenChange={setConfirmReload}>
        <DialogContent>
          <DialogTitle>{t("restartTitle")}</DialogTitle>
          <DialogDescription>{t("restartBody")}</DialogDescription>
          <DialogFooter>
            <Button
              className="min-h-11"
              variant="outline"
              onClick={() => setConfirmReload(false)}
            >
              {t("keepEditing")}
            </Button>
            <Button className="min-h-11" onClick={reload}>
              {t("restart")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
