"use client";
import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useQuery } from "@tanstack/react-query";
import { z } from "zod";
import { RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { TooltipProvider } from "@/components/ui/tooltip";
import { LiquidGlass } from "@/components/ui/liquid-glass";
import { StudioBackdrop } from "@/components/avatar-viewer/studio-backdrop";
import type { EnvironmentStatus } from "@/components/avatar-viewer/environment/environment-state";
import { environmentScenes } from "@/components/avatar-viewer/environment/scenes";
import { routes } from "@/config/routes";
import type { TransportEnd } from "../transport/interview-transport";
import { useSessionUser } from "@/features/auth/components/session-gate";
import { interviewKeys } from "../api/keys";
import { getRoomDetails } from "../api/room-details";
import { useMicrophone } from "../audio/use-microphone";
import type {
  AvatarPerformance,
  AvatarDirector,
  PerformanceState,
} from "@/components/avatar-viewer/avatar-director";
import { useInterviewerSpeech } from "@/components/avatar-viewer/lipsync/use-interviewer-speech";
import type { InterviewState } from "../machine/interview-machine";
import {
  InterviewerPlaybackProvider,
  useInterviewerPlayback,
  useInterviewerPlaybackController,
  useInterviewerPlaybackStatus,
} from "../playback/interviewer-playback";
import {
  useInterviewRecording,
  type RecordingMode,
} from "../recording/use-interview-recording";
import type { SessionRecorderSnapshot } from "../recording/session-recorder";
import { useInterviewSession } from "../session/use-interview-session";
import type { CameraSnapshot } from "../video/camera-capture";
import { useCamera } from "../video/use-camera";
import { InterviewerAvatar } from "./interviewer-avatar";
import { RoomTranscript } from "./room-transcript";
import { RoomAnswer } from "./room-answer";
import { RoomStatus } from "./room-status";
import { RoomDock } from "./room-controls";
import { RoomLeave } from "./room-leave";
import { roomGlass } from "./room-control-button";
import { RoomSelfView } from "./room-self-view";
import {
  RecordingGate,
  RecordingIndicator,
  RecordingNotice,
  RecordingUploadPanel,
} from "./room-recording";
import {
  defaultRoomBackground,
  roomBackgrounds,
  type RoomBackgroundId,
} from "./room-backgrounds";
import type { CameraView } from "./room-audio-settings";

/** Ends after which the session may still be running on the server, so Leave can still end it. A
 * completed session, a closed recruitment window and a session this account cannot open cannot. */
const leavableEnds: ReadonlySet<TransportEnd["reason"]> = new Set([
  "superseded",
  "protocol",
  "gave-up",
]);

/** Shown after completion so the Candidate sees it before the report opens. */
const REPORT_REDIRECT_MS = 1500;
const scripted =
  process.env.NODE_ENV !== "production" &&
  process.env.NEXT_PUBLIC_INTERVIEW_TRANSPORT === "scripted";

export function InterviewRoom({
  interviewId,
  performance,
  onDirectorReady,
}: {
  performance?: AvatarPerformance;
  onDirectorReady?: (director: AvatarDirector | null) => void;
  interviewId: string;
}) {
  const user = useSessionUser();
  return (
    <InterviewerPlaybackProvider key={`${user.id}:${interviewId}`}>
      <LiveRoom
        userId={user.id}
        interviewId={interviewId}
        performance={performance}
        onDirectorReady={onDirectorReady}
      />
    </InterviewerPlaybackProvider>
  );
}

/** The recording's camera, shown in the self-view of a recruitment room. */
function recordingSelfView(recorder: SessionRecorderSnapshot): CameraSnapshot {
  return {
    status:
      recorder.status === "recording"
        ? "active"
        : recorder.status === "requesting"
          ? "requesting"
          : recorder.status === "error"
            ? "error"
            : "off",
    stream: recorder.stream,
    requestPending: recorder.status === "requesting",
    error: recorder.error,
    code: null,
  };
}

function performanceState(
  phase: InterviewState,
  playing: boolean,
): PerformanceState {
  if (playing) return "speaking";
  switch (phase) {
    case "LISTENING":
      return "listening";
    case "ASSET_LOADING":
    case "PROCESSING_ANSWER":
    case "TRANSITIONING":
      return "processing";
    default:
      return "idle";
  }
}

function LiveRoom({
  userId,
  interviewId,
  performance: suppliedPerformance,
  onDirectorReady,
}: {
  performance?: AvatarPerformance;
  onDirectorReady?: (director: AvatarDirector | null) => void;
  userId: string;
  interviewId: string;
}) {
  const t = useTranslations("candidate.interviewRoom");
  const router = useRouter();
  const validId = z.uuid().safeParse(interviewId).success;
  const details = useQuery({
    queryKey: [...interviewKeys.detail(interviewId), "room-context", userId],
    queryFn: ({ signal }) => getRoomDetails(interviewId, signal),
    enabled: validId,
    retry: false,
  });
  const [transcriptOpen, setTranscriptOpen] = useState(true);
  const [expanded, setExpanded] = useState(false);
  const [background, setBackground] = useState<RoomBackgroundId>(
    defaultRoomBackground,
  );
  const [sceneStatus, setSceneStatus] = useState<EnvironmentStatus>({
    state: "loading",
    progress: 0,
  });
  const [cameraView, setCameraView] = useState<CameraView>("conversation");
  const [reset, setReset] = useState(0);
  const [sceneReady, setSceneReady] = useState(false);
  const camera = useCamera();
  const microphone = useMicrophone();
  // Recruitment only: the recording starts before the interview, which waits for it. Practice
  // never records and never calls the recording API.
  const recordingMode: RecordingMode =
    details.data?.type !== "recruitment"
      ? "off"
      : details.data.completed_at === null
        ? "live"
        : "finished";
  const recording = useInterviewRecording({
    interviewId,
    ownerId: userId,
    mode: recordingMode,
    micMuted: microphone.snapshot.muted,
  });
  const sessionEnabled =
    validId &&
    details.data !== undefined &&
    (recordingMode !== "live" || recording.recorder.started);
  const { view, session } = useInterviewSession(interviewId, sessionEnabled);
  const playbackController = useInterviewerPlaybackController();
  const playback = useInterviewerPlayback();
  const playbackStatus = useInterviewerPlaybackStatus();
  const candidateSpeaking = microphone.snapshot.activity === "speaking";
  const state = performanceState(view.phase, playback.playing);
  // A turn without audio keeps the mouth released instead of flapping on text.
  const spokenTurn =
    playback.turn?.audio_status === "ready" ? playback.turn : null;
  const { mouth, speech } = useInterviewerSpeech(playback.audio, spokenTurn);
  const livePerformance = useMemo<AvatarPerformance>(
    () => ({
      state,
      candidateSpeaking,
      emotion: state === "speaking" ? { happy: 0.15 } : { relaxed: 0.2 },
      mouth,
      speech,
    }),
    [state, candidateSpeaking, mouth, speech],
  );
  const performance = suppliedPerformance ?? livePerformance;
  const completed = view.phase === "COMPLETED";
  const { finish: finishRecording, pause: pauseRecording } = recording;
  const recordingOn = recording.enabled;
  const interrupted = view.end !== null && view.end.reason !== "completed";
  useEffect(() => {
    if (recordingOn && completed) void finishRecording();
  }, [recordingOn, completed, finishRecording]);
  useEffect(() => {
    if (recordingOn && interrupted) pauseRecording();
  }, [recordingOn, interrupted, pauseRecording]);
  // A recruitment report opens once its recording is uploaded; until then the room shows the upload.
  const leaveForReport =
    completed && (!recordingOn || recording.upload.status === "ready");
  useEffect(() => {
    if (!leaveForReport) return;
    const timer = window.setTimeout(
      () => router.replace(routes.reports.detail(interviewId)),
      REPORT_REDIRECT_MS,
    );
    return () => window.clearTimeout(timer);
  }, [leaveForReport, interviewId, router]);
  const sessionType = view.sessionType ?? details.data?.type ?? null;
  const showCaption =
    view.turn !== null &&
    (view.phase === "INTERVIEWER_SPEAKING" || view.phase === "LISTENING");
  const selectedBackground =
    roomBackgrounds.find((option) => option.id === background) ??
    roomBackgrounds[0];
  const available = validId && details.data !== undefined && !details.isError;
  const canLeave =
    available &&
    details.data.completed_at === null &&
    !completed &&
    (view.end === null || leavableEnds.has(view.end.reason));
  const environment = selectedBackground.scene
    ? environmentScenes[selectedBackground.scene]
    : undefined;
  // Without an interviewer canvas a 3D room can't render, so its still stays.
  const roomSceneStatus: EnvironmentStatus = !environment
    ? { state: "off" }
    : available
      ? sceneStatus
      : !validId || details.isError
        ? { state: "fallback", reason: "unsupported" }
        : { state: "loading", progress: 0 };
  return (
    <TooltipProvider delayDuration={250}>
      <section
        aria-label={t("label")}
        data-background={background}
        data-scene-status={roomSceneStatus.state}
        data-performance-state={performance.state}
        className="fixed inset-0 z-30 isolate overflow-hidden bg-neutral-900 text-white [--panel-translate-y:8px] [--panel-blur:0px] [--panel-open-dur:240ms] [--panel-close-dur:180ms]"
      >
        <h1 className="sr-only">{t("label")}</h1>
        <div aria-label={t("stage")} className="absolute inset-0">
          {available ? (
            <InterviewerAvatar
              performance={performance}
              onDirectorReady={onDirectorReady}
              userId={userId}
              session={details.data}
              atmosphere="warm"
              cameraView={cameraView}
              viewResetSignal={reset}
              onReadyChange={setSceneReady}
              backgroundSrc={selectedBackground.image}
              environment={environment}
              onEnvironmentStatusChange={setSceneStatus}
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
            <span>{t(`type.${sessionType ?? "unknown"}`)}</span>
            {view.questionNumber !== null && view.questionCount !== null && (
              <span className="rounded-full border border-white/15 px-2 py-0.5 text-[10px] text-white/80 tabular-nums">
                {t("progress", {
                  position: view.questionNumber,
                  count: view.questionCount,
                })}
              </span>
            )}
            {view.turn?.kind === "follow_up" &&
              view.turn.position === view.position && (
                <span
                  data-testid="follow-up-label"
                  className="rounded-full border border-white/25 bg-white/10 px-2 py-0.5 text-[10px] text-white"
                >
                  {t("followUp.label")}
                </span>
              )}
            {recordingOn && recordingMode === "live" && !completed && (
              <RecordingIndicator
                recording={recording.recorder.status === "recording"}
              />
            )}
            {scripted && (
              <span className="rounded-full border border-white/15 px-2 py-0.5 text-[10px] text-white/65">
                {t("preview")}
              </span>
            )}
          </div>
          <RoomLeave
            live={canLeave}
            sessionType={sessionType}
            submitting={view.leaving === "submitting"}
            onLeave={() => {
              void session.leave();
            }}
          />
        </header>
        {(details.isError || !validId) && (
          <div
            role="alert"
            className={`absolute top-20 left-4 z-30 max-w-sm space-y-3 rounded-2xl p-4 text-sm ${roomGlass}`}
          >
            <p>{validId ? t("detailsError") : t("invalidLink")}</p>
            {validId && (
              <Button
                variant="secondary"
                onClick={() => {
                  void details.refetch();
                }}
              >
                <RotateCcw aria-hidden />
                {t("retryInterviewer")}
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
            entries={view.transcript}
            recruitment={sessionType === "recruitment"}
            expanded={expanded}
            onExpand={() => setExpanded((value) => !value)}
          />
        </LiquidGlass>
        <div
          className={`absolute right-3 bottom-[104px] left-3 z-30 flex justify-center md:right-6 md:bottom-32 md:left-6 ${transcriptOpen ? "md:pl-[384px] lg:pr-[280px] xl:pl-[414px] 2xl:pl-[444px]" : "lg:pr-[280px]"}`}
        >
          <div className="flex w-full max-w-[520px] flex-col gap-3">
            {recordingMode === "live" && !recording.recorder.started && (
              <RecordingGate
                recorder={recording.recorder}
                onRetry={recording.retryCamera}
              />
            )}
            {recordingMode === "live" &&
              recording.recorder.started &&
              !completed &&
              !interrupted && (
                <RecordingNotice
                  recorder={recording.recorder}
                  onResume={recording.retryCamera}
                />
              )}
            {validId && (
              <RoomStatus
                view={view}
                interviewId={interviewId}
                playback={playbackStatus}
                onPlay={() => playbackController.resume()}
                onRetry={() => {
                  if (recordingMode === "live") recording.retryCamera();
                  session.retry();
                }}
                completion={
                  recordingOn ? (
                    <RecordingUploadPanel
                      upload={recording.upload}
                      interviewId={interviewId}
                      onRetry={recording.retryUpload}
                      onGiveUp={recording.giveUp}
                    />
                  ) : undefined
                }
              />
            )}
            {showCaption && view.turn && (
              <figure
                className={`rounded-2xl px-4 py-3 text-sm leading-relaxed ${roomGlass}`}
              >
                <figcaption className="sr-only">{t("caption")}</figcaption>
                <p aria-live="polite" lang="en">
                  {view.turn.text}
                </p>
              </figure>
            )}
            {validId && (
              <RoomAnswer
                view={view}
                session={session}
                microphone={microphone}
              />
            )}
          </div>
        </div>
        <RoomSelfView
          camera={
            recordingOn
              ? recordingSelfView(recording.recorder)
              : camera.snapshot
          }
          hideOnMobile={transcriptOpen && expanded}
        />
        <RoomDock
          microphone={microphone}
          transcriptOpen={transcriptOpen}
          onTranscriptToggle={() => setTranscriptOpen((value) => !value)}
          camera={camera}
          background={background}
          onBackgroundChange={setBackground}
          sceneStatus={roomSceneStatus}
          cameraView={cameraView}
          onCameraViewChange={setCameraView}
          sceneReady={available && sceneReady}
          onReset={() => setReset((value) => value + 1)}
          recordingCamera={
            recordingOn
              ? {
                  status: recording.recorder.status,
                  live:
                    recordingMode === "live" &&
                    !completed &&
                    !interrupted &&
                    recording.recorder.started &&
                    !recording.recorder.limitReached,
                  onResume: recording.retryCamera,
                }
              : undefined
          }
        />
      </section>
    </TooltipProvider>
  );
}
