"use client";
import { useState } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { z } from "zod";
import { ArrowUpRight, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { TooltipProvider } from "@/components/ui/tooltip";
import { LiquidGlass } from "@/components/ui/liquid-glass";
import { StudioBackdrop } from "@/components/avatar-viewer/studio-backdrop";
import { routes } from "@/config/routes";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { interviewKeys } from "../api/keys";
import { getRoomDetails } from "../api/room-details";
import { useCamera } from "../video/use-camera";
import { InterviewerAvatar } from "./interviewer-avatar";
import { RoomTranscript, type TranscriptEntry } from "./room-transcript";
import { RoomDock } from "./room-controls";
import { roomGlass } from "./room-control-button";
import { RoomSelfView } from "./room-self-view";
import { roomBackgrounds, type RoomBackgroundId } from "./room-backgrounds";
import type { CameraView } from "./room-audio-settings";

export function InterviewRoom({
  interviewId,
  transcriptEntries,
}: {
  interviewId: string;
  transcriptEntries?: readonly TranscriptEntry[];
}) {
  const user = useSessionUser();
  return (
    <RoomPreview
      key={`${user.id}:${interviewId}`}
      userId={user.id}
      interviewId={interviewId}
      transcriptEntries={transcriptEntries}
    />
  );
}
function RoomPreview({
  userId,
  interviewId,
  transcriptEntries,
}: {
  userId: string;
  interviewId: string;
  transcriptEntries?: readonly TranscriptEntry[];
}) {
  const validId = z.uuid().safeParse(interviewId).success;
  const details = useQuery({
    queryKey: [...interviewKeys.detail(interviewId), "room-context", userId],
    queryFn: ({ signal }) => getRoomDetails(interviewId, signal),
    enabled: validId,
    retry: false,
  });
  const [transcriptOpen, setTranscriptOpen] = useState(true);
  const [expanded, setExpanded] = useState(false);
  const [background, setBackground] = useState<RoomBackgroundId>("office");
  const [cameraView, setCameraView] = useState<CameraView>("conversation");
  const [reset, setReset] = useState(0);
  const [sceneReady, setSceneReady] = useState(false);
  const camera = useCamera();
  const selectedBackground =
    roomBackgrounds.find((option) => option.id === background) ??
    roomBackgrounds[0];
  const available = validId && details.data !== undefined && !details.isError;
  return (
    <TooltipProvider delayDuration={250}>
      <section
        aria-label="Interview room"
        data-background={background}
        className="fixed inset-0 z-30 isolate overflow-hidden bg-neutral-900 text-white [--panel-translate-y:8px] [--panel-blur:0px] [--panel-open-dur:240ms] [--panel-close-dur:180ms]"
      >
        <h1 className="sr-only">Interview room</h1>
        <div aria-label="Interview stage" className="absolute inset-0">
          {available ? (
            <InterviewerAvatar
              userId={userId}
              session={details.data}
              atmosphere="warm"
              cameraView={cameraView}
              viewResetSignal={reset}
              onReadyChange={setSceneReady}
              backgroundSrc={selectedBackground.image}
              horizontalOffset={transcriptOpen ? 0.13 : 0}
            />
          ) : (
            <StudioBackdrop immersive src={selectedBackground.image} />
          )}
        </div>
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 bg-[linear-gradient(180deg,#11131835_0%,transparent_25%,transparent_70%,#11131855_100%)]"
        />
        <header className="absolute top-[max(16px,env(safe-area-inset-top))] right-4 left-4 z-20 flex items-start justify-between gap-3 md:top-6 md:right-6 md:left-6">
          <div className="flex h-10 items-center gap-2 rounded-full border border-white/15 bg-neutral-900/55 px-4 text-xs shadow-lg backdrop-blur-xl">
            <span>
              {details.data?.type === "practice"
                ? "Practice room"
                : details.data?.type === "recruitment"
                  ? "Recruitment room"
                  : "Voice interview"}
            </span>
            <span className="rounded-full border border-white/15 px-2 py-0.5 text-[10px] text-white/65">
              Preview
            </span>
          </div>
          <Button
            asChild
            variant="ghost"
            className="h-10 rounded-full border border-white/15 bg-neutral-900/55 px-4 text-xs text-white shadow-lg backdrop-blur-xl hover:bg-neutral-900/80 hover:text-white"
          >
            <Link href={routes.history}>
              Leave
              <ArrowUpRight aria-hidden className="size-3.5" />
            </Link>
          </Button>
        </header>
        {(details.isError || !validId) && (
          <div
            role="alert"
            className={`absolute top-20 left-4 z-30 max-w-sm space-y-3 rounded-2xl p-4 text-sm ${roomGlass}`}
          >
            <p>
              {validId
                ? "Unable to load interview details. Your interviewer is unavailable."
                : "This interview room link is invalid."}
            </p>
            {validId && (
              <Button
                variant="secondary"
                onClick={() => {
                  void details.refetch();
                }}
              >
                <RotateCcw aria-hidden />
                Retry interviewer
              </Button>
            )}
          </div>
        )}
        <LiquidGlass
          id="room-transcript"
          data-open={transcriptOpen}
          inert={!transcriptOpen}
          aria-hidden={!transcriptOpen}
          variant="frosted"
          blur={16}
          refraction={5}
          dispersion={0}
          specular={0.12}
          tint="rgba(24,26,31,.68)"
          style={{
            transitionProperty: "top, transform, opacity",
            filter: transcriptOpen ? "none" : undefined,
            willChange: "transform",
          }}
          className={`t-panel-slide absolute ${expanded ? "top-20" : "top-[40%] max-md:[@media(max-height:740px)]:top-[34%]"} right-3 bottom-[110px] left-3 z-20 flex min-h-0 flex-col overflow-hidden rounded-[26px] text-white [--glass-foreground:#fafafa] md:top-20 md:right-auto md:bottom-32 md:left-6 md:w-[360px] xl:w-[390px] 2xl:w-[420px]`}
        >
          <RoomTranscript
            entries={transcriptEntries}
            expanded={expanded}
            onExpand={() => setExpanded((value) => !value)}
          />
        </LiquidGlass>
        <RoomSelfView
          camera={camera.snapshot}
          hideOnMobile={transcriptOpen && expanded}
        />
        <RoomDock
          transcriptOpen={transcriptOpen}
          onTranscriptToggle={() => setTranscriptOpen((value) => !value)}
          camera={camera}
          background={background}
          onBackgroundChange={setBackground}
          cameraView={cameraView}
          onCameraViewChange={setCameraView}
          sceneReady={available && sceneReady}
          onReset={() => setReset((value) => value + 1)}
        />
      </section>
    </TooltipProvider>
  );
}
