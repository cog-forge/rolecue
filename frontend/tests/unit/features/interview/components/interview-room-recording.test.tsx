import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import axios from "axios";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { InterviewRoom } from "@/features/interview/components/interview-room";
import { IntlWrapper } from "@tests/helpers/intl";
import { deferred } from "../audio/media-fakes";
import {
  asStream,
  FakeAvStream,
  FakeMediaRecorder,
  installRecordingFakes,
  namedError,
} from "../recording/recording-fakes";
import {
  FakeSocket,
  serverMessage,
  syncData,
  textTurn,
} from "../transport/fake-socket";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
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
  AvatarViewer: () => null,
}));
const id = "aaaaaaaa-aaaa-4aaa-8aaa-000000000087";
const ok = (data: unknown) => ({ data: { success: true, data } });
const session = (
  type: "practice" | "recruitment",
  completed_at: string | null = null,
) => ({
  id,
  type,
  avatar_id: null,
  completed_at,
});
const recordingPath = `/interviews/${id}/recording`;
function serveApi({
  type = "recruitment",
  completedAt = null,
  recordStatus = "none",
}: {
  type?: "practice" | "recruitment";
  completedAt?: string | null;
  recordStatus?: string;
} = {}) {
  mocks.get.mockImplementation(async (path: string) =>
    path === recordingPath
      ? ok({ record_status: recordStatus })
      : ok(session(type, completedAt)),
  );
}
function mount() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <IntlWrapper locale="en">
        <InterviewRoom interviewId={id} />
      </IntlWrapper>
    </QueryClientProvider>,
  );
}
let seq = 0;
function serve(socket: FakeSocket, type: string, data: unknown) {
  act(() => socket.receive(serverMessage(type, data, ++seq)));
}
async function connected() {
  await waitFor(() => expect(FakeSocket.instances).toHaveLength(1));
  const socket = FakeSocket.latest();
  act(() => socket.open());
  return socket;
}
async function complete(socket: FakeSocket) {
  serve(socket, "session.sync", syncData({ session_type: "recruitment" }));
  serve(socket, "interviewer.turn", textTurn(1));
  serve(socket, "state", { state: "listening", position: 1 });
  await screen.findByText("Your turn to answer");
  serve(socket, "state", { state: "transitioning", position: null });
  serve(socket, "state", { state: "completed", position: null });
  serve(socket, "session.completed", { evaluation_status: "pending" });
  act(() => socket.drop(1000));
}
const recordingCalls = () =>
  [...mocks.get.mock.calls, ...mocks.post.mock.calls].filter(([path]) =>
    String(path).includes("/recording"),
  );

let media: ReturnType<typeof installRecordingFakes>;
beforeEach(() => {
  vi.resetAllMocks();
  vi.stubEnv("NEXT_PUBLIC_API_BASE_URL", "http://localhost:8080");
  seq = 0;
  FakeSocket.reset();
  vi.stubGlobal("WebSocket", FakeSocket);
  media = installRecordingFakes();
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

it("starts a recruitment interview only once the camera recording runs, and discloses it", async () => {
  serveApi();
  const camera = deferred<MediaStream>();
  media.getUserMedia.mockReturnValueOnce(camera.promise);
  mount();
  expect(await screen.findByText("Turn on your camera to start")).toBeVisible();
  expect(screen.getByText("This interview is recorded")).toBeVisible();
  expect(
    screen.getByText(
      "Your camera and microphone are recorded for the whole interview.",
    ),
  ).toBeVisible();
  expect(
    screen.getByText(
      "Only the recruiter of this job posting can watch the recording.",
    ),
  ).toBeVisible();
  expect(
    screen.getByText("It is deleted 90 days after the hiring decision."),
  ).toBeVisible();
  expect(
    screen.getByText("Waiting for camera and microphone access…"),
  ).toBeVisible();
  await act(async () => new Promise((resolve) => setTimeout(resolve, 50)));
  expect(FakeSocket.instances).toHaveLength(0);
  camera.resolve(asStream(new FakeAvStream()));
  await connected();
  expect(media.getUserMedia).toHaveBeenCalledWith(
    expect.objectContaining({
      video: expect.anything(),
      audio: expect.anything(),
    }),
  );
  expect(FakeMediaRecorder.instances).toHaveLength(1);
  expect(screen.getByText("Recording")).toBeVisible();
  expect(
    screen.queryByText("Turn on your camera to start"),
  ).not.toBeInTheDocument();
  expect(
    screen.getByRole("button", { name: "Camera on for the recording" }),
  ).toBeDisabled();
  expect(document.body.textContent).not.toMatch(/eKYC|identity|cheat/i);
});

it("keeps a recruitment interview closed while the camera is denied, then starts once allowed", async () => {
  serveApi();
  media.getUserMedia.mockRejectedValueOnce(namedError("NotAllowedError"));
  mount();
  expect(
    await screen.findByText(
      "Camera or microphone access is blocked. Allow both for this site in your browser settings, then try again.",
    ),
  ).toBeVisible();
  expect(FakeSocket.instances).toHaveLength(0);
  fireEvent.click(screen.getByRole("button", { name: "Try again" }));
  await connected();
  expect(FakeMediaRecorder.instances).toHaveLength(1);
});

it("explains a missing camera and does not start the interview", async () => {
  serveApi();
  media.getUserMedia.mockRejectedValue(namedError("NotFoundError"));
  mount();
  expect(
    await screen.findByText(
      "No camera or microphone was found. Connect one, then try again.",
    ),
  ).toBeVisible();
  await act(async () => new Promise((resolve) => setTimeout(resolve, 50)));
  expect(FakeSocket.instances).toHaveLength(0);
});

it("never records a practice interview or calls the recording API", async () => {
  serveApi({ type: "practice" });
  mount();
  expect(await screen.findByText("Practice room")).toBeVisible();
  const socket = await connected();
  serve(socket, "session.sync", syncData());
  serve(socket, "session.completed", { evaluation_status: "pending" });
  act(() => socket.drop(1000));
  expect(await screen.findByText("Interview complete")).toBeVisible();
  expect(media.getUserMedia).not.toHaveBeenCalled();
  expect(FakeMediaRecorder.instances).toHaveLength(0);
  expect(recordingCalls()).toEqual([]);
  expect(
    screen.queryByText("This interview is recorded"),
  ).not.toBeInTheDocument();
});

it("uploads the recording after the interview, shows its progress, and opens the report once ready", async () => {
  serveApi();
  mocks.post.mockImplementation(async (path: string) =>
    path === recordingPath
      ? ok({
          upload_url: "https://storage.example.test/put",
          method: "PUT",
          headers: { "Content-Type": "video/webm" },
          expires_at: "2026-11-03T08:15:00Z",
          record_status: "uploading",
        })
      : ok({ record_status: "ready" }),
  );
  let finishPut: () => void = () => undefined;
  const put = vi.spyOn(axios, "request").mockImplementation(
    (config) =>
      new Promise((resolve) => {
        config.onUploadProgress?.({
          loaded: 1,
          total: 2,
          bytes: 1,
          lengthComputable: true,
        });
        finishPut = () => resolve({ status: 200 });
      }),
  );
  mount();
  const socket = await connected();
  FakeMediaRecorder.latest().emit([1, 2, 3]);
  await complete(socket);
  expect(
    await screen.findByText("Uploading your recording… 50%"),
  ).toBeVisible();
  expect(
    screen.getByText("Keep this page open until the upload finishes."),
  ).toBeVisible();
  expect(put).toHaveBeenCalledWith(
    expect.objectContaining({
      url: "https://storage.example.test/put",
      method: "PUT",
      withCredentials: false,
    }),
  );
  expect(mocks.replace).not.toHaveBeenCalled();
  act(() => finishPut());
  expect(await screen.findByText("Your recording was uploaded.")).toBeVisible();
  expect(mocks.post).toHaveBeenCalledWith(`${recordingPath}/complete`);
  await waitFor(
    () => expect(mocks.replace).toHaveBeenCalledWith(`/reports/${id}`),
    {
      timeout: 3000,
    },
  );
  expect(
    recordingCalls().every(([path]) => String(path).startsWith(recordingPath)),
  ).toBe(true);
});

it("reopened after completion, finishes an upload the browser still holds", async () => {
  serveApi({ completedAt: "2026-11-03T08:20:00Z", recordStatus: "ready" });
  mount();
  const socket = await connected();
  serve(socket, "session.completed", { evaluation_status: "pending" });
  act(() => socket.drop(1000));
  expect(await screen.findByText("Your recording was uploaded.")).toBeVisible();
  // A completed interview never opens the camera again.
  expect(media.getUserMedia).not.toHaveBeenCalled();
  expect(mocks.post).not.toHaveBeenCalled();
});
