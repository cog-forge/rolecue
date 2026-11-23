import { useQuery } from "@tanstack/react-query";
import {
  getSavedSetup,
  savedSetupKeys,
  type SetupTarget,
} from "@/features/interview-setup/api/saved-setup";
import {
  roomBackgrounds,
  type RoomBackgroundId,
} from "../components/room-backgrounds";
import type { RoomDetails } from "./room-details";

/** The room a stored environment names, or null when there is none or this build doesn't know it. */
export function roomBackgroundOf(
  environment: string | null | undefined,
): RoomBackgroundId | null {
  return roomBackgrounds.find((room) => room.id === environment)?.id ?? null;
}

/** The saved setup a session was started from, if it carries one. */
export function setupTargetOf(
  details: Pick<RoomDetails, "type" | "role_profile_id" | "application_id">,
): SetupTarget | null {
  if (details.type === "practice" && details.role_profile_id)
    return { mode: "practice", roleProfileId: details.role_profile_id };
  if (details.type === "recruitment" && details.application_id)
    return { mode: "recruitment", applicationId: details.application_id };
  return null;
}

/** The room saved in the session's setup. The room reads it once to choose where to open; a change
 * made inside the room is a view choice for this visit and is never written back to the setup.
 * `pending` holds the stage back so it never shows the default room before the saved one. */
export function useSavedRoom(userId: string, details: RoomDetails | undefined) {
  const target = details ? setupTargetOf(details) : null;
  const setup = useQuery({
    queryKey: target
      ? savedSetupKeys.detail(userId, target)
      : ["interview-room", "no-saved-setup"],
    queryFn: ({ signal }) =>
      target ? getSavedSetup(target, signal) : Promise.reject(new Error()),
    enabled: target !== null,
    // Read once: the room never moves under the candidate after it has opened.
    staleTime: Infinity,
    retry: false,
    refetchOnWindowFocus: false,
  });
  return {
    pending: target !== null && setup.isPending,
    background: roomBackgroundOf(setup.data?.environment),
  };
}
