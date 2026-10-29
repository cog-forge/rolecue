"use client";
import { Settings2, RotateCcw } from "lucide-react";
import { Toolbar } from "radix-ui";
import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { useMicrophone } from "../audio/use-microphone";
import { roomGlass, roomIconButton } from "./room-control-button";

export type CameraView = "conversation" | "close";
export function RoomAudioSettings({
  microphone,
  cameraView,
  onCameraViewChange,
  onReset,
  sceneReady,
}: {
  microphone: ReturnType<typeof useMicrophone>;
  cameraView: CameraView;
  onCameraViewChange: (view: CameraView) => void;
  onReset: () => void;
  sceneReady: boolean;
}) {
  const state = microphone.snapshot;
  const active = state.stream !== null;
  const selectedMissing =
    state.selectedDeviceId !== "default" &&
    !state.devices.some((device) => device.deviceId === state.selectedDeviceId);
  return (
    <Popover>
      <Tooltip>
        <TooltipTrigger asChild>
          <PopoverTrigger asChild>
            <Toolbar.Button asChild>
              <Button
                variant="ghost"
                size="icon"
                aria-label="Audio settings"
                className={roomIconButton}
              >
                <Settings2
                  aria-hidden
                  className="size-[45%]"
                  strokeWidth={1.5}
                />
              </Button>
            </Toolbar.Button>
          </PopoverTrigger>
        </TooltipTrigger>
        <TooltipContent side="top">Audio settings</TooltipContent>
      </Tooltip>
      <PopoverContent
        aria-label="Audio settings"
        side="top"
        sideOffset={16}
        collisionPadding={12}
        className={`w-72 space-y-4 rounded-2xl p-4 motion-reduce:animate-none! ${roomGlass}`}
      >
        <div className="flex items-center gap-2 text-sm font-medium">
          <Settings2 aria-hidden className="size-4" />
          Audio settings
        </div>
        <div className="space-y-2">
          <label
            htmlFor="room-microphone-input"
            className="text-xs text-white/75"
          >
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
              id="room-microphone-input"
              className="h-11 w-full border-white/20 bg-white/5 text-white"
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
                .filter((device) => device.deviceId !== "default")
                .map((device) => (
                  <SelectItem key={device.deviceId} value={device.deviceId}>
                    {device.label}
                  </SelectItem>
                ))}
            </SelectContent>
          </Select>
        </div>
        <p className="text-xs leading-relaxed text-white/65">
          Your microphone stays on this device during this preview.
        </p>
        {state.actualDeviceLabel && (
          <p className="text-xs text-white/75">
            Current input: {state.actualDeviceLabel}
          </p>
        )}
        {state.analysisWarning && (
          <div className="space-y-2">
            <p role="status" className="text-xs leading-relaxed text-white/75">
              {state.analysisWarning}
            </p>
            <Button variant="secondary" onClick={microphone.retryAnalysis}>
              Retry input meter
            </Button>
          </div>
        )}
        {state.deviceWarning && (
          <div className="space-y-2">
            <p role="status" className="text-xs text-white/75">
              {state.deviceWarning}
            </p>
            <Button
              variant="secondary"
              onClick={() => {
                void microphone.refreshDevices();
              }}
            >
              Refresh inputs
            </Button>
          </div>
        )}
        {state.readiness === "interrupted" && (
          <Button
            variant="secondary"
            onClick={() => {
              void microphone.retry();
            }}
          >
            Retry microphone
          </Button>
        )}
        <Button
          variant="secondary"
          disabled={!active && !state.requestPending}
          onClick={microphone.stop}
        >
          Stop microphone
        </Button>
        <div className="space-y-3 border-t border-white/10 pt-4">
          <h3 className="text-xs text-white/70">Interviewer view</h3>
          <RadioGroup
            aria-label="Interviewer view"
            value={cameraView}
            disabled={!sceneReady}
            onValueChange={(value) => {
              if (value === "conversation" || value === "close")
                onCameraViewChange(value);
            }}
            className="flex gap-4"
          >
            <label className="flex items-center gap-2 text-xs">
              <RadioGroupItem
                value="conversation"
                className="border-white/40"
              />
              Conversation
            </label>
            <label className="flex items-center gap-2 text-xs">
              <RadioGroupItem value="close" className="border-white/40" />
              Closer
            </label>
          </RadioGroup>
          <Button
            variant="secondary"
            disabled={!sceneReady}
            onClick={onReset}
            className="h-10 text-xs"
          >
            <RotateCcw aria-hidden />
            Reset view
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  );
}
