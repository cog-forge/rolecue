"use client";

import { useEffect, useId, useState } from "react";
import { useTranslations } from "next-intl";
import { Mic, MicOff } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { getBrowserCapabilities } from "@/features/interview/audio/browser-capabilities";
import type { MicrophoneSnapshot } from "@/features/interview/audio/microphone-capture";
import { useMicrophone } from "@/features/interview/audio/use-microphone";
import { microphoneMessage } from "../lib/microphone-messages";

// Order of the capability list, as getBrowserCapabilities() returns it.
const capabilityNames = [
  "microphone",
  "camera",
  "mediaRecorder",
  "webAudio",
  "webGL",
  "online",
] as const satisfies readonly (keyof ReturnType<
  typeof getBrowserCapabilities
>)[];

export type MicrophoneReadiness = MicrophoneSnapshot["readiness"];

// onReadinessChange tells the start panel whether the microphone is live right now; Start needs it
// to be "ready" at the moment it is pressed.
// releaseNote replaces the closing note when the page also checks the camera (recruitment).
export function PreflightCheck({
  onReadinessChange,
  releaseNote,
}: {
  onReadinessChange?: (readiness: MicrophoneReadiness) => void;
  releaseNote?: string;
} = {}) {
  const t = useTranslations("candidate.setup.preflight");
  const microphone = useMicrophone();
  const state = microphone.snapshot;
  useEffect(() => {
    onReadinessChange?.(state.readiness);
  }, [onReadinessChange, state.readiness]);
  const inputId = useId();
  const helpId = useId();
  const [capabilities, setCapabilities] = useState<ReturnType<
    typeof getBrowserCapabilities
  > | null>(null);
  const status =
    state.status === "requesting"
      ? t("status.requesting")
      : state.readiness === "ready"
        ? t("status.ready")
        : state.readiness === "muted"
          ? t("status.muted")
          : state.readiness === "interrupted"
            ? t("status.interrupted")
            : state.readiness === "error"
              ? t("status.error")
              : state.status === "stopped"
                ? t("status.stopped")
                : t("status.idle");
  const activity =
    state.activity === "speaking"
      ? t("activity.speaking")
      : state.activity === "silent"
        ? t("activity.silent")
        : state.activity === "unavailable"
          ? t("activity.unavailable")
          : t("activity.inactive");
  const selectedMissing =
    state.selectedDeviceId !== "default" &&
    !state.devices.some((d) => d.deviceId === state.selectedDeviceId);

  return (
    <div className="max-w-2xl space-y-8">
      <section aria-labelledby={`${inputId}-heading`} className="space-y-5">
        <div className="flex items-start gap-3">
          <div className="rounded-lg border p-2.5" aria-hidden="true">
            {state.muted ? (
              <MicOff className="size-5" />
            ) : (
              <Mic className="size-5" />
            )}
          </div>
          <div className="space-y-1">
            <h2 id={`${inputId}-heading`} className="font-semibold">
              {t("title")}
            </h2>
            <p className="text-sm text-muted-foreground">{t("intro")}</p>
          </div>
        </div>

        <div className="space-y-2">
          <label htmlFor={inputId} className="text-sm font-medium">
            {t("inputLabel")}
          </label>
          <Select
            value={state.selectedDeviceId}
            onValueChange={(id) => {
              void microphone.selectDevice(id);
            }}
            disabled={state.requestPending}
          >
            <SelectTrigger
              id={inputId}
              aria-describedby={helpId}
              className="h-11 w-full min-w-0"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="default">{t("systemDefault")}</SelectItem>
              {selectedMissing && (
                <SelectItem value={state.selectedDeviceId} disabled>
                  {t("selectedMissing")}
                </SelectItem>
              )}
              {state.devices
                .filter((d) => d.deviceId !== "default")
                .map((device) => (
                  <SelectItem key={device.deviceId} value={device.deviceId}>
                    {device.label}
                  </SelectItem>
                ))}
            </SelectContent>
          </Select>
          <p id={helpId} className="text-xs text-muted-foreground">
            {state.actualDeviceLabel
              ? t("currentInput", { label: state.actualDeviceLabel })
              : t("inputsHint")}
          </p>
        </div>

        <div className="space-y-3 rounded-lg border bg-muted/30 p-4">
          <p role="status" className="text-sm font-medium">
            {status}
          </p>
          <div className="flex items-center justify-between gap-3 text-xs text-muted-foreground">
            <span>{t("inputLevel")}</span>
            <span>{activity}</span>
          </div>
          <meter
            aria-label={t("meterLabel")}
            min={0}
            max={1}
            value={state.level}
            className="block h-3 w-full"
          />
          <p className="text-xs text-muted-foreground">{t("quietNote")}</p>
        </div>

        {state.error && (
          <p
            role="alert"
            className="rounded-lg border border-destructive/40 p-3 text-sm text-destructive"
          >
            {microphoneMessage(state.error.message, t)}
          </p>
        )}
        {state.readiness === "interrupted" && (
          <p className="text-sm text-muted-foreground">{t("interrupted")}</p>
        )}
        {state.requestPending && state.status === "stopped" && (
          <p role="status" className="text-sm text-muted-foreground">
            {t("cancelled")}
          </p>
        )}
        {state.deviceWarning && (
          <div className="space-y-2">
            <p role="status" className="text-sm text-muted-foreground">
              {microphoneMessage(state.deviceWarning, t)}
            </p>
            <Button
              className="h-auto min-h-11 whitespace-normal py-2"
              variant="outline"
              onClick={() => {
                void microphone.refreshDevices();
              }}
            >
              {t("refreshInputs")}
            </Button>
          </div>
        )}
        {state.analysisWarning && (
          <div className="space-y-2">
            <p role="status" className="text-sm text-muted-foreground">
              {microphoneMessage(state.analysisWarning, t)}
            </p>
            <Button
              className="h-auto min-h-11 whitespace-normal py-2"
              variant="outline"
              onClick={microphone.retryAnalysis}
            >
              {t("retryAudio")}
            </Button>
          </div>
        )}

        <div className="flex flex-wrap gap-2">
          <Button
            className="h-auto min-h-11 whitespace-normal py-2"
            disabled={
              state.requestPending ||
              (state.stream !== null && state.readiness !== "interrupted")
            }
            onClick={() => {
              void (state.error || state.stream
                ? microphone.retry()
                : microphone.start());
            }}
          >
            {state.status === "requesting"
              ? t("button.checking")
              : state.error || state.readiness === "interrupted"
                ? t("button.retry")
                : t("button.test")}
          </Button>
          {state.stream && (
            <>
              <Button
                className="h-auto min-h-11 whitespace-normal py-2"
                variant="outline"
                aria-pressed={state.muted}
                onClick={() => microphone.setMuted(!state.muted)}
              >
                {state.muted ? t("button.unmute") : t("button.mute")}
              </Button>
              <Button
                className="h-auto min-h-11 whitespace-normal py-2"
                variant="outline"
                onClick={microphone.stop}
              >
                {t("button.stop")}
              </Button>
            </>
          )}
          {state.status === "requesting" && (
            <Button
              className="h-auto min-h-11 whitespace-normal py-2"
              variant="outline"
              onClick={microphone.stop}
            >
              {t("button.cancel")}
            </Button>
          )}
        </div>
        <p className="text-xs text-muted-foreground">
          {releaseNote ?? t("releaseNote")}
        </p>
      </section>

      <section
        aria-label={t("capabilities.label")}
        className="space-y-3 border-t pt-5"
      >
        <Button
          className="h-auto min-h-11 whitespace-normal py-2"
          variant="outline"
          onClick={() => setCapabilities(getBrowserCapabilities())}
        >
          {t("capabilities.check")}
        </Button>
        <p className="text-sm text-muted-foreground">
          {t("capabilities.note")}
        </p>
        <div aria-live="polite">
          {capabilities && (
            <dl className="grid grid-cols-2 gap-3 text-sm">
              {capabilityNames.map((name) => (
                <div key={name}>
                  <dt className="font-medium">
                    {t(`capabilities.name.${name}`)}
                  </dt>
                  <dd className="text-muted-foreground">
                    {capabilities[name]
                      ? t("capabilities.available")
                      : t("capabilities.unavailable")}
                  </dd>
                </div>
              ))}
            </dl>
          )}
        </div>
      </section>
    </div>
  );
}
