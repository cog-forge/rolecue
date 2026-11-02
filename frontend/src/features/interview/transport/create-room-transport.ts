import { getApiBaseUrl } from "@/lib/api/config";
import { createScriptedTransport } from "./scripted-transport";
import { checkInterviewSession } from "./session-check";
import {
  createWebSocketTransport,
  liveInterviewUrl,
} from "./websocket-transport";

/** The live socket in every build. `NEXT_PUBLIC_INTERVIEW_TRANSPORT=scripted` swaps in the scripted
 * mock, but only outside production builds, so it can never ship.
 */
export function createRoomTransport(interviewId: string) {
  if (
    process.env.NODE_ENV !== "production" &&
    process.env.NEXT_PUBLIC_INTERVIEW_TRANSPORT === "scripted"
  )
    return createScriptedTransport({ sessionId: interviewId });
  return createWebSocketTransport({
    url: liveInterviewUrl(getApiBaseUrl() ?? "", interviewId),
    checkSession: (signal) => checkInterviewSession(interviewId, signal),
  });
}
