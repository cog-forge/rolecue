import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, afterEach, expect, it, vi } from "vitest";
import { InterviewRoom } from "@/features/interview/components/interview-room";
import { installMediaFakes } from "../audio/media-fakes";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  viewer: vi.fn(),
}));
vi.mock("@/lib/api/client", () => ({
  apiClient: { get: mocks.get, post: mocks.post },
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => ({ id: "candidate-1" }),
}));
vi.mock("@/components/avatar-viewer/avatar-viewer", () => ({
  AvatarViewer: (props: { fallbackReason?: string }) => {
    mocks.viewer(props);
    return <p>{props.fallbackReason}</p>;
  },
}));
const id = "aaaaaaaa-aaaa-4aaa-8aaa-000000000053";
const details = {
  id,
  type: "practice",
  avatar_id: null,
  current_question: { position: 1, content: "Legacy text question" },
  turns: [{ position: 1, question: "Old question", answer: "Old answer" }],
  answered_count: 1,
  question_count: 8,
  completed_at: "2026-10-29T09:00:00Z",
};
const ok = (data: unknown) => ({ data: { success: true, data } });
function mount() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <InterviewRoom interviewId={id} />
    </QueryClientProvider>,
  );
}
beforeEach(() => {
  vi.resetAllMocks();
  mocks.get.mockResolvedValue(ok(details));
});
afterEach(() => vi.unstubAllGlobals());

it("renders the voice room UI without questions, answer forms, progress or session writes", async () => {
  mount();
  expect(await screen.findByText("Practice room")).toBeVisible();
  expect(screen.getByText("Preview")).toBeVisible();
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  expect(screen.queryByText("Legacy text question")).not.toBeInTheDocument();
  expect(screen.queryByText("Old answer")).not.toBeInTheDocument();
  expect(
    mocks.get.mock.calls.every(([path]) => path === `/interviews/${id}`),
  ).toBe(true);
  expect(mocks.post).not.toHaveBeenCalled();
});
it("shows a read-only transcript panel, with no fabricated conversation", async () => {
  mount();
  await screen.findByText("Practice room");
  expect(screen.getByRole("region", { name: "Live transcript" })).toBeVisible();
  expect(
    screen.getByText("Live captions are unavailable in this preview."),
  ).toBeVisible();
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Hide transcript" }));
  expect(
    screen.queryByRole("region", { name: "Live transcript" }),
  ).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Show transcript" }));
  expect(screen.getByRole("region", { name: "Live transcript" })).toBeVisible();
  expect(mocks.post).not.toHaveBeenCalled();
});
it("only acquires audio on demand, toggles the real track, and releases it on unmount", async () => {
  const { media, tracks } = installMediaFakes();
  const view = mount();
  await screen.findByText("Practice room");
  expect(media.getUserMedia).not.toHaveBeenCalled();
  const renders = mocks.viewer.mock.calls.length;
  fireEvent.click(screen.getByRole("button", { name: "Enable microphone" }));
  await screen.findByRole("button", { name: "Mute microphone" });
  expect(media.getUserMedia).toHaveBeenCalledExactlyOnceWith({
    audio: true,
    video: false,
  });
  expect(tracks[0].enabled).toBe(true);
  fireEvent.click(screen.getByRole("button", { name: "Mute microphone" }));
  expect(tracks[0].enabled).toBe(false);
  fireEvent.click(screen.getByRole("button", { name: "Unmute microphone" }));
  expect(tracks[0].enabled).toBe(true);
  expect(mocks.viewer).toHaveBeenCalledTimes(renders);
  view.unmount();
  expect(tracks[0].stop).toHaveBeenCalledTimes(1);
  expect(mocks.post).not.toHaveBeenCalled();
});
it("shows permission refusal, supports retry, and can stop microphone capture", async () => {
  const { media, tracks } = installMediaFakes();
  media.getUserMedia.mockRejectedValueOnce(
    new DOMException("Denied", "NotAllowedError"),
  );
  mount();
  await screen.findByText("Practice room");
  fireEvent.click(screen.getByRole("button", { name: "Enable microphone" }));
  expect(
    await screen.findByText(/Microphone access is not allowed/),
  ).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "Retry microphone" }));
  await screen.findByRole("button", { name: "Mute microphone" });
  fireEvent.click(screen.getByRole("button", { name: "Audio settings" }));
  fireEvent.click(screen.getByRole("button", { name: "Stop microphone" }));
  expect(tracks[0].stop).toHaveBeenCalledTimes(1);
  expect(
    screen.getByRole("button", { name: "Enable microphone" }),
  ).toBeVisible();
});
it("keeps recruitment avatar private and leaves preview controls usable when details fail", async () => {
  mocks.get.mockResolvedValue(
    ok({
      ...details,
      type: "recruitment",
      avatar_id: "bbbbbbbb-bbbb-4bbb-8bbb-000000000001",
    }),
  );
  mount();
  expect(await screen.findByText("Recruitment room")).toBeVisible();
  expect(
    mocks.get.mock.calls.every(([path]) => !path.includes("/inventory")),
  ).toBe(true);
  expect(mocks.post).not.toHaveBeenCalled();
});
it("refetches metadata only when Retry interviewer is requested", async () => {
  mocks.get.mockRejectedValueOnce(new Error("Offline"));
  mount();
  expect(
    await screen.findByText(/Unable to load interview details/),
  ).toBeVisible();
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Retry interviewer" }));
  });
  await waitFor(() => expect(screen.getByText("Practice room")).toBeVisible());
  expect(mocks.post).not.toHaveBeenCalled();
});
