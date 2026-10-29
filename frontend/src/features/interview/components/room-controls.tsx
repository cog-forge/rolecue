"use client";
import {
  Mic,
  MicOff,
  MessageSquare,
  Video,
  VideoOff,
  LoaderCircle,
} from "lucide-react";
import { Toolbar } from "radix-ui";
import { useReducedMotion } from "motion/react";
import { Dock, DockIcon } from "@/components/ui/dock";
import { AIVoiceInput } from "@/components/ui/ai-voice-input";
import { useMicrophone } from "../audio/use-microphone";
import type { useCamera } from "../video/use-camera";
import { RoomControlButton, roomGlass } from "./room-control-button";
import { RoomBackgroundPicker } from "./room-background-picker";
import { RoomAudioSettings, type CameraView } from "./room-audio-settings";
import type { RoomBackgroundId } from "./room-backgrounds";

export function RoomDock({
  transcriptOpen,
  onTranscriptToggle,
  camera,
  background,
  onBackgroundChange,
  cameraView,
  onCameraViewChange,
  sceneReady,
  onReset,
}: {
  transcriptOpen: boolean;
  onTranscriptToggle: () => void;
  camera: ReturnType<typeof useCamera>;
  background: RoomBackgroundId;
  onBackgroundChange: (id: RoomBackgroundId) => void;
  cameraView: CameraView;
  onCameraViewChange: (view: CameraView) => void;
  sceneReady: boolean;
  onReset: () => void;
}) {
  const microphone = useMicrophone();
  const state = microphone.snapshot;
  const reduced = useReducedMotion();
  const active = state.stream !== null;
  const micLabel = state.requestPending
    ? "Requesting microphone access"
    : active
      ? state.muted
        ? "Unmute microphone"
        : "Mute microphone"
      : state.error
        ? "Retry microphone"
        : "Enable microphone";
  const cameraLabel =
    camera.snapshot.status === "active"
      ? "Turn camera off"
      : camera.snapshot.status === "requesting"
        ? "Cancel camera request"
        : camera.snapshot.requestPending
          ? "Waiting for camera permission"
          : camera.snapshot.error
            ? "Retry camera"
            : "Turn camera on";
  const micWarning =
    state.error?.message ??
    (state.readiness === "interrupted"
      ? "Your microphone input was interrupted. Check Audio settings."
      : null);
  const warnings = [camera.snapshot.error, micWarning].filter(
    (warning): warning is string => Boolean(warning),
  );
  return (
    <div className="absolute bottom-[max(14px,env(safe-area-inset-bottom))] left-1/2 z-40 -translate-x-1/2 md:bottom-6">
      <Toolbar.Root asChild aria-label="Interview controls">
        <Dock
          iconSize={44}
          iconMagnification={reduced ? 44 : 58}
          direction="middle"
          className={`m-0 h-[72px] max-w-[calc(100vw-24px)] gap-1 rounded-[24px] px-2 py-2 md:gap-2 md:px-3 [&_[data-slot=dock-icon]]:p-0! max-md:[&_[data-slot=dock-icon]]:size-11! ${roomGlass}`}
        >
          <DockIcon>
            <RoomControlButton
              icon={MessageSquare}
              label={transcriptOpen ? "Hide transcript" : "Show transcript"}
              aria-pressed={transcriptOpen}
              aria-controls="room-transcript"
              onClick={onTranscriptToggle}
            />
          </DockIcon>
          <DockIcon>
            <RoomControlButton
              icon={
                camera.snapshot.status === "requesting"
                  ? LoaderCircle
                  : camera.snapshot.stream
                    ? Video
                    : VideoOff
              }
              label={cameraLabel}
              aria-pressed={camera.snapshot.stream !== null}
              disabled={
                camera.snapshot.requestPending &&
                camera.snapshot.status !== "requesting"
              }
              onClick={() => {
                if (
                  camera.snapshot.status === "active" ||
                  camera.snapshot.status === "requesting"
                )
                  camera.stop();
                else void camera.start();
              }}
            />
          </DockIcon>
          <DockIcon>
            <RoomControlButton
              icon={
                state.requestPending
                  ? LoaderCircle
                  : active && !state.muted
                    ? Mic
                    : MicOff
              }
              label={micLabel}
              disabled={state.requestPending}
              aria-pressed={active && !state.muted}
              onClick={() => {
                if (active) microphone.setMuted(!state.muted);
                else if (state.error) void microphone.retry();
                else void microphone.start();
              }}
            />
          </DockIcon>
          <div className="mx-1 flex w-12 shrink-0 flex-col items-center gap-1 md:mx-2 md:w-28">
            <AIVoiceInput
              active={state.readiness === "ready"}
              level={state.level}
            />
            <span
              role="status"
              className="text-[9px] whitespace-nowrap text-white/65"
            >
              {state.requestPending
                ? "Allow mic"
                : state.error
                  ? "Mic error"
                  : active
                    ? state.muted
                      ? "Muted"
                      : "Mic on"
                    : "Mic off"}
            </span>
          </div>
          <Toolbar.Separator className="mx-0.5 h-6 w-px shrink-0 bg-white/15" />
          <DockIcon>
            <RoomBackgroundPicker
              value={background}
              onChange={onBackgroundChange}
            />
          </DockIcon>
          <DockIcon>
            <RoomAudioSettings
              microphone={microphone}
              cameraView={cameraView}
              onCameraViewChange={onCameraViewChange}
              sceneReady={sceneReady}
              onReset={onReset}
            />
          </DockIcon>
        </Dock>
      </Toolbar.Root>
      {warnings.length > 0 && (
        <div
          role="alert"
          className={`absolute bottom-full left-1/2 mb-4 w-[min(320px,calc(100vw-24px))] -translate-x-1/2 space-y-2 rounded-2xl p-4 text-xs leading-relaxed ${roomGlass}`}
        >
          {warnings.map((warning) => (
            <p key={warning}>{warning}</p>
          ))}
        </div>
      )}
    </div>
  );
}
