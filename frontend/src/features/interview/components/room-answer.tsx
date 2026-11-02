"use client";
import { useEffect, useId, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { Keyboard, Mic, RotateCcw, Send, Square } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { encodeAnswerAudio } from "../answer/answer-audio";
import { useAnswerRecorder } from "../answer/use-answer-recorder";
import type { useMicrophone } from "../audio/use-microphone";
import type {
  InterviewSession,
  InterviewSessionView,
} from "../session/interview-session";
import {
  MAX_ANSWER_AUDIO_BYTES,
  MAX_ANSWER_DURATION_MS,
  MAX_ANSWER_TEXT_LENGTH,
} from "../transport/protocol";
import { roomGlass } from "./room-control-button";

type Notice = "timeLimit" | "audio" | "rejected" | null;
type IssueKey =
  | "STT_NO_SPEECH"
  | "STT_FAILED"
  | "ANSWER_TOO_LONG"
  | "AUDIO"
  | "VALIDATION_ERROR"
  | "INTERNAL_ERROR"
  | "generic";
const issueKeys: Partial<Record<string, IssueKey>> = {
  STT_NO_SPEECH: "STT_NO_SPEECH",
  STT_FAILED: "STT_FAILED",
  ANSWER_TOO_LONG: "ANSWER_TOO_LONG",
  AUDIO_FORMAT_UNSUPPORTED: "AUDIO",
  AUDIO_TOO_LARGE: "AUDIO",
  AUDIO_CHUNK_ORDER: "AUDIO",
  VALIDATION_ERROR: "VALIDATION_ERROR",
  INTERNAL_ERROR: "INTERNAL_ERROR",
};
/** Localized copy is chosen by the server's error code, never its message. */
const issueKey = (code: string): IssueKey => issueKeys[code] ?? "generic";

export function formatCountdown(ms: number) {
  const seconds = Math.max(0, Math.ceil(ms / 1000));
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;
}

/** Answering controls for the current question. The typed answer is always offered. */
export function RoomAnswer({
  view,
  session,
  microphone,
}: {
  view: InterviewSessionView;
  session: InterviewSession;
  microphone: ReturnType<typeof useMicrophone>;
}) {
  const [typing, setTyping] = useState(false);
  // A new question starts with a fresh recorder, draft and notices.
  return (
    <AnswerTurn
      key={view.position ?? "none"}
      view={view}
      session={session}
      microphone={microphone}
      typing={typing}
      onTypingChange={setTyping}
    />
  );
}

function AnswerTurn({
  view,
  session,
  microphone,
  typing,
  onTypingChange,
}: {
  view: InterviewSessionView;
  session: InterviewSession;
  microphone: ReturnType<typeof useMicrophone>;
  typing: boolean;
  onTypingChange: (typing: boolean) => void;
}) {
  const t = useTranslations("candidate.interviewRoom");
  const textId = useId();
  const [draft, setDraft] = useState("");
  const [notice, setNotice] = useState<Notice>(null);
  const { snapshot: recorded, recorder } = useAnswerRecorder((recording) => {
    void encodeAnswerAudio(recording.blob, recording.mimeType).then(
      (answer) => {
        if (answer.bytes > MAX_ANSWER_AUDIO_BYTES) setNotice("audio");
        else if (session.submitAudio(answer))
          setNotice(recording.reason === "time-limit" ? "timeLimit" : null);
        // The question moved on or an earlier answer is still pending: say so, never drop it silently.
        else setNotice("rejected");
      },
    );
  });
  const mic = microphone.snapshot;
  const listening = view.phase === "LISTENING";
  const answerable =
    (listening || view.phase === "RECONNECTING") &&
    !view.sending &&
    !view.canResend;
  const recording = recorded.status === "recording";
  const micReady = mic.readiness === "ready" && mic.stream !== null;
  const ended =
    view.phase === "COMPLETED" ||
    view.phase === "TERMINATED_EARLY" ||
    view.phase === "FATAL_ERROR";

  // With the microphone on, the answer starts recording when the server listens; again after a
  // retryable failure. "Answer by voice" re-arms it.
  const armed = useRef(true);
  const wasSending = useRef(view.sending);
  useEffect(() => {
    // Not after INTERNAL_ERROR: that answer is still pending and is sent again as is.
    if (wasSending.current && !view.sending && !view.canResend)
      armed.current = true;
    wasSending.current = view.sending;
  }, [view.sending, view.canResend]);
  useEffect(() => {
    if (!armed.current || !listening || typing || view.sending) return;
    if (view.canResend) return;
    if (!micReady || recording || !mic.stream) return;
    armed.current = false;
    recorder.start(mic.stream);
  }, [
    listening,
    typing,
    view.sending,
    view.canResend,
    micReady,
    recording,
    mic.stream,
    recorder,
  ]);
  useEffect(() => {
    if (ended) recorder.cancel();
  }, [ended, recorder]);

  function answerByVoice() {
    armed.current = true;
    setNotice(null);
    onTypingChange(false);
    if (mic.stream && mic.readiness === "muted") microphone.setMuted(false);
    else if (!mic.stream) void microphone.start();
    else if (micReady && listening && !recording) {
      armed.current = false;
      recorder.start(mic.stream);
    }
  }
  function typeInstead() {
    recorder.cancel();
    onTypingChange(true);
  }
  function sendText() {
    if (session.submitText(draft)) setDraft("");
  }

  if (ended || view.phase === "IDLE" || view.phase === "CONNECTING")
    return null;
  const status = view.sending
    ? view.phase === "RECONNECTING"
      ? t("answer.held")
      : view.phase === "PROCESSING_ANSWER"
        ? t("answer.processing")
        : t("answer.sending")
    : view.issue
      ? t(`issue.${issueKey(view.issue)}`)
      : notice === "timeLimit"
        ? t("answer.timeLimit")
        : notice === "audio"
          ? t("issue.AUDIO")
          : notice === "rejected"
            ? t("issue.generic")
            : recorded.error === "unsupported"
              ? t("answer.recorderUnsupported")
              : recorded.error === "failed"
                ? t("answer.recorderFailed")
                : listening && !typing && !recording && !micReady
                  ? t("answer.micNeeded")
                  : null;
  const remaining = MAX_ANSWER_DURATION_MS - recorded.elapsedMs;
  const trimmed = draft.trim();

  return (
    <section
      aria-label={t("answer.yourTurn")}
      className={`w-full space-y-3 rounded-[22px] p-3 text-sm ${roomGlass}`}
    >
      {listening && !view.sending && (
        <p className="px-1 text-xs font-medium text-white/80">
          {t("answer.yourTurn")}
        </p>
      )}
      {status && (
        <p
          role={view.issue || notice || recorded.error ? "alert" : "status"}
          className="px-1 text-xs leading-relaxed text-white/85"
        >
          {status}
        </p>
      )}
      {view.canResend && (
        <Button
          type="button"
          variant="secondary"
          onClick={() => session.resend()}
        >
          <RotateCcw aria-hidden />
          {t("answer.resend")}
        </Button>
      )}
      {typing ? (
        <form
          className="space-y-2"
          onSubmit={(event) => {
            event.preventDefault();
            sendText();
          }}
        >
          <label htmlFor={textId} className="sr-only">
            {t("answer.textLabel")}
          </label>
          <Textarea
            id={textId}
            value={draft}
            maxLength={MAX_ANSWER_TEXT_LENGTH}
            placeholder={t("answer.textPlaceholder")}
            onChange={(event) => setDraft(event.target.value)}
            className="max-h-40 min-h-20 border-white/15 bg-neutral-950/40 text-white placeholder:text-white/45"
          />
          <div className="flex flex-wrap items-center justify-between gap-2">
            <span className="text-[11px] text-white/55 tabular-nums">
              {t("answer.textHint", {
                count: draft.length,
                max: MAX_ANSWER_TEXT_LENGTH,
              })}
            </span>
            <div className="flex gap-2">
              <Button
                type="button"
                variant="ghost"
                onClick={answerByVoice}
                className="text-white hover:bg-white/10 hover:text-white"
              >
                <Mic aria-hidden />
                {t("answer.startVoice")}
              </Button>
              <Button type="submit" disabled={!answerable || !trimmed}>
                <Send aria-hidden />
                {t("answer.send")}
              </Button>
            </div>
          </div>
        </form>
      ) : (
        <div className="flex flex-wrap items-center gap-2">
          {recording ? (
            <>
              <span className="sr-only">{t("answer.recording")}</span>
              <span
                role="timer"
                aria-label={t("answer.timerLabel")}
                className="flex h-9 items-center gap-2 rounded-full bg-red-500/20 px-3 text-xs font-medium text-white tabular-nums"
              >
                <span
                  aria-hidden
                  className="size-2 rounded-full bg-red-400 motion-safe:animate-pulse"
                />
                {t("answer.timeLeft", { time: formatCountdown(remaining) })}
              </span>
              <Button type="button" onClick={() => recorder.finish()}>
                <Square aria-hidden />
                {t("answer.done")}
              </Button>
            </>
          ) : (
            <Button
              type="button"
              disabled={!answerable || mic.requestPending}
              onClick={answerByVoice}
            >
              <Mic aria-hidden />
              {t("answer.startVoice")}
            </Button>
          )}
          <Button
            type="button"
            variant="ghost"
            onClick={typeInstead}
            className="text-white hover:bg-white/10 hover:text-white"
          >
            <Keyboard aria-hidden />
            {t("answer.typeInstead")}
          </Button>
        </div>
      )}
    </section>
  );
}
