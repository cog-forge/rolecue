import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { useState, type ReactNode } from "react";
import { beforeEach, expect, it, vi } from "vitest";
import type { RoomDetails } from "@/features/interview/api/room-details";
import {
  roomBackgroundOf,
  setupTargetOf,
  useSavedRoom,
} from "@/features/interview/api/saved-room";
import {
  envelope,
  practiceView,
  profileId,
  recruitmentView,
  applicationId,
} from "../../interview-setup/saved-setup-fixtures";

const get = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api/client", () => ({ apiClient: { get } }));
beforeEach(() => get.mockReset());

const details = (patch: Partial<RoomDetails>): RoomDetails => ({
  id: "aaaaaaaa-aaaa-4aaa-8aaa-000000000053",
  type: "practice",
  avatar_id: null,
  role_profile_id: null,
  application_id: null,
  completed_at: null,
  ...patch,
});

const Wrapper = ({ children }: { children: ReactNode }) => {
  const [client] = useState(() => new QueryClient());
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
};

it("maps a stored environment to a known room and ignores anything else", () => {
  expect(roomBackgroundOf("office-3d")).toBe("office-3d");
  expect(roomBackgroundOf("balcony")).toBe("balcony");
  expect(roomBackgroundOf("moon-base")).toBeNull();
  expect(roomBackgroundOf(null)).toBeNull();
  expect(roomBackgroundOf(undefined)).toBeNull();
});

it("finds the saved setup of a practice or recruitment session", () => {
  expect(setupTargetOf(details({ role_profile_id: profileId }))).toEqual({
    mode: "practice",
    roleProfileId: profileId,
  });
  expect(
    setupTargetOf(
      details({ type: "recruitment", application_id: applicationId }),
    ),
  ).toEqual({ mode: "recruitment", applicationId });
  expect(setupTargetOf(details({}))).toBeNull();
  // A reference of the other mode is not this session's setup.
  expect(setupTargetOf(details({ application_id: applicationId }))).toBeNull();
});

it("opens on the room saved in a practice setup", async () => {
  get.mockResolvedValue(envelope(practiceView({ environment: "meeting-3d" })));
  const { result } = renderHook(
    () => useSavedRoom("u1", details({ role_profile_id: profileId })),
    { wrapper: Wrapper },
  );
  expect(result.current.pending).toBe(true);
  await waitFor(() => expect(result.current.pending).toBe(false));
  expect(result.current.background).toBe("meeting-3d");
  expect(get).toHaveBeenCalledWith(
    `/interview-setups/practice/${profileId}`,
    expect.anything(),
  );
});

it("reads the recruitment setup of an application", async () => {
  get.mockResolvedValue(envelope(recruitmentView({ environment: "balcony" })));
  const { result } = renderHook(
    () =>
      useSavedRoom(
        "u1",
        details({ type: "recruitment", application_id: applicationId }),
      ),
    { wrapper: Wrapper },
  );
  await waitFor(() => expect(result.current.background).toBe("balcony"));
});

it("falls back when the saved room is not one this build knows", async () => {
  get.mockResolvedValue(envelope(practiceView({ environment: "moon-base" })));
  const unknown = renderHook(
    () => useSavedRoom("u1", details({ role_profile_id: profileId })),
    { wrapper: Wrapper },
  );
  await waitFor(() => expect(unknown.result.current.pending).toBe(false));
  expect(unknown.result.current.background).toBeNull();
});

it("falls back when the setup cannot be read", async () => {
  get.mockResolvedValue({ data: { success: false } });
  const failed = renderHook(
    () => useSavedRoom("u1", details({ role_profile_id: profileId })),
    { wrapper: Wrapper },
  );
  await waitFor(() => expect(failed.result.current.pending).toBe(false));
  expect(failed.result.current.background).toBeNull();
});

it("asks for nothing while the session is unknown or has no setup", () => {
  const unknown = renderHook(() => useSavedRoom("u1", undefined), {
    wrapper: Wrapper,
  });
  const none = renderHook(() => useSavedRoom("u1", details({})), {
    wrapper: Wrapper,
  });
  expect(unknown.result.current).toEqual({ pending: false, background: null });
  expect(none.result.current).toEqual({ pending: false, background: null });
  expect(get).not.toHaveBeenCalled();
});
