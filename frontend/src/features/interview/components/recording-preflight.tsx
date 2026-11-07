"use client";
import { useEffect, useId, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { Video, VideoOff } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { CONSENT_VERSION } from "../recording/recording-consent";
import { useCamera } from "../video/use-camera";
import { RecordingDisclosure } from "./recording-disclosure";

/**
 * The camera check of a recruitment preflight. Start waits until the camera is live here; the room
 * then opens the recording itself. The preview is released when the Candidate leaves the page.
 */
export function RecordingPreflight({
  onReadyChange,
  onConsentChange,
}: {
  onReadyChange?: (ready: boolean) => void;
  /** The recording consent box (KAN-297). Start stays off until it is ticked. */
  onConsentChange?: (accepted: boolean) => void;
}) {
  const t = useTranslations("candidate.interviewRoom.recording");
  const ids = useId();
  const [consented, setConsented] = useState(false);
  const changeConsent = (checked: boolean) => {
    setConsented(checked);
    onConsentChange?.(checked);
  };
  const camera = useCamera();
  const state = camera.snapshot;
  const ready = state.status === "active";
  useEffect(() => {
    onReadyChange?.(ready);
  }, [onReadyChange, ready]);
  const video = useRef<HTMLVideoElement>(null);
  useEffect(() => {
    const element = video.current;
    if (!element) return;
    element.srcObject = state.stream;
    return () => {
      element.srcObject = null;
    };
  }, [state.stream]);
  return (
    <section
      aria-labelledby={`${ids}-heading`}
      className="mt-8 space-y-5 border-t pt-6"
    >
      <div className="flex items-start gap-3">
        <div className="rounded-lg border p-2.5" aria-hidden="true">
          {ready ? (
            <Video className="size-5" />
          ) : (
            <VideoOff className="size-5" />
          )}
        </div>
        <div className="space-y-1">
          <h2 id={`${ids}-heading`} className="font-semibold">
            {t("preflight.title")}
          </h2>
          <p className="text-sm text-muted-foreground">
            {t("preflight.intro")}
          </p>
        </div>
      </div>
      <RecordingDisclosure className="rounded-lg border bg-muted/30 p-4 text-sm" />
      <div className="space-y-2 rounded-lg border p-4 text-sm">
        <div className="flex items-start gap-3">
          <Checkbox
            id={`${ids}-consent`}
            checked={consented}
            onCheckedChange={(checked) => changeConsent(checked === true)}
            aria-describedby={`${ids}-consent-version`}
            className="mt-0.5"
          />
          <label htmlFor={`${ids}-consent`} className="min-h-6">
            {t("preflight.consent.label")}
          </label>
        </div>
        <p
          id={`${ids}-consent-version`}
          className="pl-7 text-xs text-muted-foreground"
        >
          {t("preflight.consent.version", { version: CONSENT_VERSION })}
        </p>
        {!consented && (
          <p role="status" className="pl-7 text-xs text-muted-foreground">
            {t("preflight.consent.declined")}
          </p>
        )}
      </div>
      <div className="aspect-video w-full max-w-sm overflow-hidden rounded-lg border bg-muted">
        {state.stream ? (
          <video
            ref={video}
            aria-label={t("preflight.previewLabel")}
            autoPlay
            playsInline
            muted
            className="size-full -scale-x-100 object-cover"
          />
        ) : (
          <div className="grid size-full place-items-center">
            <VideoOff aria-hidden className="size-6 text-muted-foreground" />
          </div>
        )}
      </div>
      <p role="status" className="text-sm font-medium">
        {state.status === "requesting"
          ? t("preflight.checking")
          : ready
            ? t("preflight.ready")
            : t("preflight.idle")}
      </p>
      {state.code && (
        <p
          role="alert"
          className="rounded-lg border border-destructive/40 p-3 text-sm text-destructive"
        >
          {t(`error.${state.code}`)}
        </p>
      )}
      <div className="flex flex-wrap gap-2">
        {ready ? (
          <Button
            className="h-auto min-h-11 whitespace-normal py-2"
            variant="outline"
            onClick={camera.stop}
          >
            {t("preflight.stop")}
          </Button>
        ) : (
          <Button
            className="h-auto min-h-11 whitespace-normal py-2"
            disabled={state.requestPending}
            onClick={() => void camera.start()}
          >
            {state.status === "requesting"
              ? t("preflight.checking")
              : state.code
                ? t("preflight.retry")
                : t("preflight.test")}
          </Button>
        )}
      </div>
    </section>
  );
}
