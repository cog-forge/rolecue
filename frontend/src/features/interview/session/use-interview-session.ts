"use client";

import { useEffect, useState, useSyncExternalStore } from "react";
import { useInterviewerPlaybackController } from "../playback/interviewer-playback";
import { createRoomTransport } from "../transport/create-room-transport";
import { createInterviewSession } from "./interview-session";

/** One live session per mounted room. Unmount disconnects and silences it. */
export function useInterviewSession(interviewId: string, enabled = true) {
  const playback = useInterviewerPlaybackController();
  const [session] = useState(() =>
    createInterviewSession({
      transport: createRoomTransport(interviewId),
      playback,
    }),
  );
  useEffect(() => {
    if (!enabled) return;
    session.start();
    return () => session.stop();
  }, [session, enabled]);
  const view = useSyncExternalStore(
    session.subscribe,
    session.getSnapshot,
    session.getSnapshot,
  );
  return { view, session };
}
