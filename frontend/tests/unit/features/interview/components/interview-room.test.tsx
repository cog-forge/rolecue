import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, afterEach, expect, it, vi } from "vitest";
import type { AvatarPerformance } from "@/components/avatar-viewer/avatar-director";
import { InterviewRoom } from "@/features/interview/components/interview-room";
import type { AppLocale } from "@/lib/i18n/config";
import { IntlWrapper } from "@tests/helpers/intl";
import { installMediaFakes } from "../audio/media-fakes";
import {
  FakeSocket,
  serverMessage,
  syncData,
  textTurn,
} from "../transport/fake-socket";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  viewer: vi.fn(),
  replace: vi.fn(),
}));
vi.mock("@/lib/api/client", () => ({
  apiClient: { get: mocks.get, post: mocks.post },
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mocks.replace, push: vi.fn() }),
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
  completed_at: null,
};
const ok = (data: unknown) => ({ data: { success: true, data } });
function mount(locale: AppLocale = "en") {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <IntlWrapper locale={locale}>
        <InterviewRoom interviewId={id} />
      </IntlWrapper>
    </QueryClientProvider>,
  );
}
let seq = 0;
function serve(
  socket: FakeSocket,
  type: string,
  data: unknown,
  replyTo?: string,
) {
  act(() => socket.receive(serverMessage(type, data, ++seq, { replyTo })));
}
function sent(socket: FakeSocket, type: string) {
  return socket.sentMessages().filter((message) => message.type === type);
}
async function connected() {
  await waitFor(() => expect(FakeSocket.instances).toHaveLength(1));
  const socket = FakeSocket.latest();
  act(() => socket.open());
  return socket;
}
/** Opens a socket and brings the room to `listening` on a text-only first question. */
async function listening(
  socket: FakeSocket,
  sync: Parameters<typeof syncData>[0] = {},
) {
  serve(socket, "session.sync", syncData(sync));
  serve(socket, "interviewer.turn", textTurn(1));
  serve(socket, "state", { state: "listening", position: 1 });
  await screen.findByText("Your turn to answer");
}
beforeEach(() => {
  vi.resetAllMocks();
  vi.stubEnv("NEXT_PUBLIC_API_BASE_URL", "http://localhost:8080");
  seq = 0;
  FakeSocket.reset();
  vi.stubGlobal("WebSocket", FakeSocket);
  mocks.get.mockResolvedValue(ok(details));
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

it("connects the live socket and never uses the REST turn flow or the preview", async () => {
  mount();
  expect(await screen.findByText("Practice room")).toBeVisible();
  const socket = await connected();
  expect(socket.url).toBe(`ws://localhost:8080/interviews/${id}/live`);
  expect(socket.protocols).toEqual(["rolecue.interview.v1"]);
  expect(screen.queryByText("Preview")).not.toBeInTheDocument();
  expect(screen.queryByText("Legacy text question")).not.toBeInTheDocument();
  expect(screen.queryByText("Old answer")).not.toBeInTheDocument();
  expect(screen.getByText("Connecting to your interviewer…")).toBeVisible();
  expect(
    mocks.get.mock.calls.every(([path]) => path === `/interviews/${id}`),
  ).toBe(true);
  expect(mocks.post).not.toHaveBeenCalled();
});
it("shows the transcript panel with real captions only", async () => {
  mount();
  await screen.findByText("Practice room");
  expect(screen.getByRole("region", { name: "Live transcript" })).toBeVisible();
  expect(
    screen.getByText(
      "Questions and your answers appear here as the interview goes on.",
    ),
  ).toBeVisible();
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
it("shows the question as a caption, reports turn.played and takes a typed answer with its practice transcript", async () => {
  mount();
  const socket = await connected();
  serve(socket, "session.sync", syncData());
  serve(socket, "interviewer.turn", textTurn(1, { text: "Why Go?" }));
  expect(screen.getAllByText("Why Go?")).toHaveLength(2);
  expect(screen.getByText("Question 1 of 3")).toBeVisible();
  expect(sent(socket, "turn.played")).toEqual([
    expect.objectContaining({ data: { position: 1 } }),
  ]);
  serve(socket, "state", { state: "listening", position: 1 });
  const room = screen.getByRole("region", { name: "Interview room" });
  expect(room).toHaveAttribute("data-performance-state", "listening");
  fireEvent.click(await screen.findByRole("button", { name: "Type instead" }));
  const send = screen.getByRole("button", { name: "Send answer" });
  expect(send).toBeDisabled();
  fireEvent.change(screen.getByLabelText("Your answer"), {
    target: { value: "  Simple and fast.  " },
  });
  fireEvent.click(send);
  const [answer] = sent(socket, "answer.text");
  expect(answer).toMatchObject({
    v: 1,
    data: { position: 1, text: "Simple and fast." },
  });
  expect(screen.getByText("Sending your answer…")).toBeVisible();
  serve(socket, "state", { state: "processing", position: 1 });
  expect(room).toHaveAttribute("data-performance-state", "processing");
  serve(
    socket,
    "answer.accepted",
    {
      position: 1,
      answered_count: 1,
      duplicate: false,
      transcript: "Simple and fast.",
    },
    String(answer.id),
  );
  expect(screen.getByText("Simple and fast.")).toBeVisible();
  serve(socket, "state", { state: "transitioning", position: null });
  expect(screen.getByText("Preparing the next question…")).toBeVisible();
  expect(mocks.post).not.toHaveBeenCalled();
});
it("shows localized retry copy for a failed spoken answer", async () => {
  mount();
  const socket = await connected();
  await listening(socket);
  fireEvent.click(screen.getByRole("button", { name: "Type instead" }));
  fireEvent.change(screen.getByLabelText("Your answer"), {
    target: { value: "Hello" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Send answer" }));
  const [answer] = sent(socket, "answer.text");
  serve(
    socket,
    "error",
    {
      code: "STT_NO_SPEECH",
      message: "internal detail",
      retryable: true,
      fatal: false,
    },
    String(answer.id),
  );
  serve(socket, "state", { state: "listening", position: 1 });
  expect(
    screen.getByText("We didn't catch any speech. Please answer again."),
  ).toBeVisible();
  expect(screen.queryByText("internal detail")).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Send answer" })).toBeDisabled();
});
it("reconnects after a dropped socket without replaying a played turn", async () => {
  mount();
  const first = await connected();
  await listening(first);
  act(() => first.drop(1011));
  expect(screen.getByText("Connection lost. Reconnecting…")).toBeVisible();
  const room = screen.getByRole("region", { name: "Interview room" });
  expect(room).toHaveAttribute("data-performance-state", "idle");
  await waitFor(() => expect(FakeSocket.instances).toHaveLength(2));
  const second = FakeSocket.latest();
  act(() => second.open());
  serve(second, "session.sync", syncData({ resumed: true }));
  serve(second, "interviewer.turn", textTurn(1));
  const [played] = sent(first, "turn.played");
  const [again] = sent(second, "turn.played");
  expect(again).toMatchObject({ data: { position: 1 } });
  expect(again.id).not.toBe(played.id);
  serve(second, "state", { state: "listening", position: 1 });
  expect(
    screen.queryByText("Connection lost. Reconnecting…"),
  ).not.toBeInTheDocument();
  expect(screen.getAllByText("Question 1?")).toHaveLength(2);
});
it("stops for a superseded socket and reconnects only on request", async () => {
  mount();
  const socket = await connected();
  await listening(socket);
  act(() => socket.drop(4001));
  expect(
    screen.getByText("This interview continues in another window"),
  ).toBeVisible();
  expect(screen.queryByText("Your turn to answer")).not.toBeInTheDocument();
  await new Promise((resolve) => setTimeout(resolve, 600));
  expect(FakeSocket.instances).toHaveLength(1);
  expect(mocks.get).toHaveBeenCalledTimes(1);
  fireEvent.click(screen.getByRole("button", { name: "Continue here" }));
  await waitFor(() => expect(FakeSocket.instances).toHaveLength(2));
});
it("shows a stop banner, not a retry, when the session no longer accepts answers", async () => {
  mount();
  const socket = await connected();
  await listening(socket);
  serve(socket, "error", {
    code: "APPLICATION_NOT_ELIGIBLE",
    message: "closed",
    retryable: false,
    fatal: true,
  });
  act(() => socket.drop(4003));
  expect(
    screen.getByText("This interview no longer accepts answers"),
  ).toBeVisible();
  expect(
    screen.queryByRole("button", { name: "Try again" }),
  ).not.toBeInTheDocument();
  expect(
    screen.getByRole("link", { name: "Back to interview history" }),
  ).toBeVisible();
});
it("announces completion and opens the report", async () => {
  mount();
  const socket = await connected();
  await listening(socket);
  serve(socket, "state", { state: "transitioning", position: null });
  serve(socket, "state", { state: "completed", position: null });
  serve(socket, "session.completed", { evaluation_status: "pending" });
  act(() => socket.drop(1000));
  expect(screen.getByText("Interview complete")).toBeVisible();
  expect(screen.getByRole("link", { name: "View report" })).toHaveAttribute(
    "href",
    `/reports/${id}`,
  );
  await waitFor(
    () => expect(mocks.replace).toHaveBeenCalledWith(`/reports/${id}`),
    { timeout: 3000 },
  );
});
it("offers a one-tap Play when autoplay is refused, then reports the turn once it ends", async () => {
  const play = vi
    .spyOn(HTMLMediaElement.prototype, "play")
    .mockRejectedValueOnce(new DOMException("Blocked", "NotAllowedError"))
    .mockResolvedValue(undefined);
  vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => {});
  vi.spyOn(HTMLMediaElement.prototype, "load").mockImplementation(() => {});
  vi.stubGlobal(
    "URL",
    Object.assign(URL, {
      createObjectURL: vi.fn(() => "blob:turn-1"),
      revokeObjectURL: vi.fn(),
    }),
  );
  mount();
  const socket = await connected();
  serve(socket, "session.sync", syncData());
  serve(
    socket,
    "interviewer.turn",
    textTurn(1, {
      audio: { mime_type: "audio/mpeg", data: "SUQz" },
      audio_status: "ready",
    }),
  );
  const button = await screen.findByRole("button", { name: "Play question" });
  expect(sent(socket, "turn.played")).toEqual([]);
  fireEvent.click(button);
  await waitFor(() => expect(play).toHaveBeenCalledTimes(2));
  const audio = play.mock.contexts[1] as HTMLAudioElement;
  act(() => audio.onplaying?.(new Event("playing")));
  expect(
    screen.getByRole("region", { name: "Interview room" }),
  ).toHaveAttribute("data-performance-state", "speaking");
  act(() => audio.onended?.(new Event("ended")));
  expect(sent(socket, "turn.played")).toHaveLength(1);
});
it("never shows recruitment answers in the transcript", async () => {
  mocks.get.mockResolvedValue(ok({ ...details, type: "recruitment" }));
  mount();
  const socket = await connected();
  await listening(socket, { session_type: "recruitment" });
  expect(screen.getByText("The interviewer's questions")).toBeVisible();
  serve(socket, "answer.accepted", {
    position: 1,
    answered_count: 1,
    duplicate: false,
    transcript: "Leaked transcript",
  });
  expect(screen.queryByText("Leaked transcript")).not.toBeInTheDocument();
});
it("renders room chrome in Vietnamese while the question stays in English", async () => {
  mount("vi");
  expect(await screen.findByText("Phòng luyện tập")).toBeVisible();
  const socket = await connected();
  serve(socket, "session.sync", syncData());
  serve(socket, "interviewer.turn", textTurn(1, { text: "Why Go?" }));
  serve(socket, "state", { state: "listening", position: 1 });
  expect(await screen.findByText("Đến lượt bạn trả lời")).toBeVisible();
  expect(screen.getByText("Câu 1/3")).toBeVisible();
  expect(screen.getAllByText("Why Go?")[0]).toBeVisible();
});

it("drives the avatar mouth and speech motion from the turn whose audio is playing", async () => {
  const play = vi
    .spyOn(HTMLMediaElement.prototype, "play")
    .mockResolvedValue(undefined);
  vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => {});
  vi.spyOn(HTMLMediaElement.prototype, "load").mockImplementation(() => {});
  vi.stubGlobal(
    "URL",
    Object.assign(URL, {
      createObjectURL: vi.fn(() => "blob:turn-1"),
      revokeObjectURL: vi.fn(),
    }),
  );
  const lastPerformance = () =>
    (mocks.viewer.mock.lastCall?.[0] as { performance?: AvatarPerformance })
      .performance;
  mount();
  const socket = await connected();
  serve(socket, "session.sync", syncData());
  serve(socket, "interviewer.turn", textTurn(1, { text: "Text only?" }));
  await waitFor(() => expect(mocks.viewer).toHaveBeenCalled());
  expect(lastPerformance()?.speech).toBeUndefined();
  serve(
    socket,
    "interviewer.turn",
    textTurn(2, {
      text: "Why Go?",
      audio: { mime_type: "audio/mpeg", data: "SUQz" },
      audio_status: "ready",
    }),
  );
  await waitFor(() => expect(play).toHaveBeenCalled());
  const audio = play.mock.contexts.at(-1) as HTMLAudioElement;
  act(() => audio.onplaying?.(new Event("playing")));
  await waitFor(() =>
    expect(lastPerformance()).toMatchObject({
      state: "speaking",
      mouth: { sample: expect.any(Function) },
      speech: { audio, text: "Why Go?", words: [] },
    }),
  );
});

it("loads only the owned recruitment presentation grant into the viewer", async () => {
  const access = {
    url: "https://assets.example.test/company.vrm",
    method: "GET",
    headers: { "x-model": "grant" },
    expires_at: "2099-01-01T00:00:00Z",
  };
  mocks.get.mockImplementation(async (path: string) => {
    if (path === `/interviews/${id}/interviewer`)
      return ok({
        name: "Company interviewer",
        voice_name: "Company voice",
        model_access: access,
      });
    if (path === `/interviews/${id}`)
      return ok({ ...details, type: "recruitment" });
    throw new Error("Unexpected path");
  });
  mount();
  await waitFor(() =>
    expect(mocks.viewer).toHaveBeenCalledWith(
      expect.objectContaining({
        access: {
          url: access.url,
          method: "GET",
          headers: access.headers,
          expiresAt: access.expires_at,
        },
      }),
    ),
  );
  expect(
    mocks.get.mock.calls.every(([path]) => !path.includes("inventory")),
  ).toBe(true);
  expect(mocks.post).not.toHaveBeenCalled();
});

it("labels a follow-up and keeps counting core questions, in English and Vietnamese", async () => {
  for (const [locale, progress, label] of [
    ["en", "Question 1 of 3", "Follow-up"],
    ["vi", "Câu 1/3", "Câu hỏi thêm"],
  ] as const) {
    FakeSocket.reset();
    const { unmount } = mount(locale);
    const socket = await connected();
    serve(socket, "session.sync", syncData());
    serve(socket, "interviewer.turn", textTurn(1));
    expect(screen.queryByTestId("follow-up-label")).not.toBeInTheDocument();
    serve(
      socket,
      "interviewer.turn",
      textTurn(2, {
        kind: "follow_up",
        question_number: 1,
        text: "Which algorithm would you pick?",
      }),
    );
    expect(screen.getByText(progress)).toBeVisible();
    expect(screen.getByTestId("follow-up-label")).toHaveTextContent(label);
    // The follow-up question itself stays English in the Vietnamese room.
    expect(
      screen.getAllByText("Which algorithm would you pick?")[0],
    ).toBeVisible();
    serve(socket, "interviewer.turn", textTurn(3, { question_number: 2 }));
    expect(screen.queryByTestId("follow-up-label")).not.toBeInTheDocument();
    unmount();
  }
});
