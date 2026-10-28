"use client";

import { useEffect, useId, useState } from "react";
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

export type MicrophoneReadiness = MicrophoneSnapshot["readiness"];

// onReadinessChange tells the start panel whether the microphone is live right now; Start needs it
// to be "ready" at the moment it is pressed.
export function PreflightCheck({
  onReadinessChange,
}: {
  onReadinessChange?: (readiness: MicrophoneReadiness) => void;
} = {}) {
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
      ? "Waiting for microphone permission"
      : state.readiness === "ready"
        ? "Microphone ready"
        : state.readiness === "muted"
          ? "Microphone muted"
          : state.readiness === "interrupted"
            ? "Microphone interrupted"
            : state.readiness === "error"
              ? "Microphone needs attention"
              : state.status === "stopped"
                ? "Microphone check stopped"
                : "Microphone not tested";
  const activity =
    state.activity === "speaking"
      ? "Sound detected"
      : state.activity === "silent"
        ? "Listening — speak to check your input"
        : state.activity === "unavailable"
          ? "Input level unavailable"
          : "Input level inactive";
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
              Check your microphone
            </h2>
            <p className="text-sm text-muted-foreground">
              Allow access when prompted, then say a few words. This check stays
              on your device and does not start an interview or use coins.
            </p>
          </div>
        </div>

        <div className="space-y-2">
          <label htmlFor={inputId} className="text-sm font-medium">
            Microphone input
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
              <SelectItem value="default">System default</SelectItem>
              {selectedMissing && (
                <SelectItem value={state.selectedDeviceId} disabled>
                  Selected microphone (unavailable)
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
              ? `Current input: ${state.actualDeviceLabel}`
              : "Available inputs appear after you allow microphone access."}
          </p>
        </div>

        <div className="space-y-3 rounded-lg border bg-muted/30 p-4">
          <p role="status" className="text-sm font-medium">
            {status}
          </p>
          <div className="flex items-center justify-between gap-3 text-xs text-muted-foreground">
            <span>Input level</span>
            <span>{activity}</span>
          </div>
          <meter
            aria-label="Microphone input level"
            min={0}
            max={1}
            value={state.level}
            className="block h-3 w-full"
          />
          <p className="text-xs text-muted-foreground">
            Quiet moments are normal. The sound indicator only helps you check
            your audio.
          </p>
        </div>

        {state.error && (
          <p
            role="alert"
            className="rounded-lg border border-destructive/40 p-3 text-sm text-destructive"
          >
            {state.error.message}
          </p>
        )}
        {state.readiness === "interrupted" && (
          <p className="text-sm text-muted-foreground">
            Your microphone has temporarily stopped providing audio. Check your
            device or retry the microphone check.
          </p>
        )}
        {state.requestPending && state.status === "stopped" && (
          <p role="status" className="text-sm text-muted-foreground">
            Check cancelled. Close or dismiss the browser permission prompt
            before trying again. Any input it opens will be released.
          </p>
        )}
        {state.deviceWarning && (
          <div className="space-y-2">
            <p role="status" className="text-sm text-muted-foreground">
              {state.deviceWarning}
            </p>
            <Button
              className="min-h-11"
              variant="outline"
              onClick={() => {
                void microphone.refreshDevices();
              }}
            >
              Refresh inputs
            </Button>
          </div>
        )}
        {state.analysisWarning && (
          <div className="space-y-2">
            <p role="status" className="text-sm text-muted-foreground">
              {state.analysisWarning}
            </p>
            <Button
              className="min-h-11"
              variant="outline"
              onClick={microphone.retryAnalysis}
            >
              Retry audio check
            </Button>
          </div>
        )}

        <div className="flex flex-wrap gap-2">
          <Button
            className="min-h-11"
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
              ? "Checking microphone…"
              : state.error || state.readiness === "interrupted"
                ? "Retry microphone"
                : "Test microphone"}
          </Button>
          {state.stream && (
            <>
              <Button
                className="min-h-11"
                variant="outline"
                aria-pressed={state.muted}
                onClick={() => microphone.setMuted(!state.muted)}
              >
                {state.muted ? "Unmute microphone" : "Mute microphone"}
              </Button>
              <Button
                className="min-h-11"
                variant="outline"
                onClick={microphone.stop}
              >
                Stop microphone check
              </Button>
            </>
          )}
          {state.status === "requesting" && (
            <Button
              className="min-h-11"
              variant="outline"
              onClick={microphone.stop}
            >
              Cancel microphone check
            </Button>
          )}
        </div>
        <p className="text-xs text-muted-foreground">
          Stop the check or leave this page to release your microphone. No
          camera access is requested.
        </p>
      </section>

      <section
        aria-label="Other browser capabilities"
        className="space-y-3 border-t pt-5"
      >
        <Button
          className="min-h-11"
          variant="outline"
          onClick={() => setCapabilities(getBrowserCapabilities())}
        >
          Check browser capabilities
        </Button>
        <p className="text-sm text-muted-foreground">
          These checks show API availability only. Speaker playback, connection
          quality and full interview readiness still need separate checks.
        </p>
        <div aria-live="polite">
          {capabilities && (
            <dl className="grid grid-cols-2 gap-3 text-sm">
              {Object.entries(capabilities).map(([name, available]) => (
                <div key={name}>
                  <dt className="font-medium">{name}</dt>
                  <dd className="text-muted-foreground">
                    {available ? "Available" : "Unavailable"}
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
