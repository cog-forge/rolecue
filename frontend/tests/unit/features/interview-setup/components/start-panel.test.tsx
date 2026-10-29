import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import type { ReactNode } from "react";
import {
  StartPanel,
  type StartPlan,
} from "@/features/interview-setup/components/start-panel";
import type { MicrophoneReadiness } from "@/features/interview-setup/components/preflight-check";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  push: vi.fn(),
  info: vi.fn(),
  user: {
    id: "11111111-1111-4111-8111-111111111111",
    full_name: "Cam Candidate",
    email: "cam@example.com",
    role: "candidate" as const,
    email_verified: true,
    is_locked: false,
    image: null,
  },
}));
vi.mock("@/lib/api/client", () => ({
  apiClient: { get: mocks.get, post: mocks.post },
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mocks.push }),
}));
vi.mock("goey-toast", () => ({ gooeyToast: { info: mocks.info } }));

const profileId = "22222222-2222-4222-8222-222222222222";
const applicationId = "33333333-3333-4333-8333-333333333333";
const sessionId = "66666666-6666-4666-8666-666666666666";
const uuidPattern =
  /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

const practice: StartPlan = {
  kind: "new",
  request: { type: "practice", roleProfileId: profileId },
};

function session(over: Record<string, unknown> = {}) {
  return {
    id: sessionId,
    type: "practice",
    role_profile_id: profileId,
    application_id: null,
    avatar_id: null,
    voice_id: null,
    started_at: "2026-10-28T08:00:00Z",
    completed_at: null,
    evaluation_status: "pending",
    question_count: 8,
    answered_count: 0,
    ...over,
  };
}

function apiFailure(status: number, code: string, message = "server says") {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: { success: false, error: { code, message } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}

function wallet(balance: number) {
  mocks.get.mockImplementation((url: string) =>
    url === "/wallet"
      ? Promise.resolve({
          data: {
            success: true,
            data: { id: "w", user_id: mocks.user.id, balance },
          },
        })
      : Promise.reject(new Error(`unexpected GET ${url}`)),
  );
}

function renderPanel(
  plan: StartPlan,
  readiness: MicrophoneReadiness = "ready",
  children?: ReactNode,
) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const ui = (value: MicrophoneReadiness) => (
    <QueryClientProvider client={client}>
      <StartPanel
        plan={plan}
        readiness={value}
        links={{ roleProfileId: profileId, applicationId }}
        returnPath="/interviews/new/preflight"
      >
        {children}
      </StartPanel>
    </QueryClientProvider>
  );
  const view = renderWithIntl(ui(readiness));
  return {
    ...view,
    setReadiness: (value: MicrophoneReadiness) => view.rerender(ui(value)),
  };
}

const startButton = () =>
  screen.getByRole("button", { name: /start interview|resume interview/i });

beforeEach(() => {
  mocks.get.mockReset();
  mocks.post.mockReset();
  mocks.push.mockReset();
  mocks.info.mockReset();
});

describe("a new practice interview", () => {
  it("shows the price next to the server's balance and starts only with a live microphone", async () => {
    wallet(9);
    mocks.post.mockResolvedValue({
      status: 201,
      data: { success: true, data: session() },
    });
    const view = renderPanel(practice, "not-ready");
    expect(await screen.findByText("9 coins")).toBeInTheDocument();
    expect(screen.getByText("5 coins")).toBeInTheDocument();
    expect(startButton()).toHaveAccessibleName("Start interview · 5 coins");
    expect(startButton()).toBeDisabled();
    expect(
      screen.getByText("Turn on your microphone to start."),
    ).toBeInTheDocument();

    view.setReadiness("muted");
    expect(startButton()).toBeDisabled();
    expect(
      screen.getByText("Unmute your microphone to start."),
    ).toBeInTheDocument();

    view.setReadiness("ready");
    expect(startButton()).toBeEnabled();
    fireEvent.click(startButton());
    await waitFor(() =>
      expect(mocks.push).toHaveBeenCalledWith(`/interviews/${sessionId}/room`),
    );
    const [url, body, config] = mocks.post.mock.calls[0];
    expect(url).toBe("/interviews");
    expect(body).toEqual({ type: "practice", role_profile_id: profileId });
    expect(config.headers["Idempotency-Key"]).toMatch(uuidPattern);
    expect(mocks.info).not.toHaveBeenCalled();
  });

  it("does not offer Start when the balance is short, and opens Add coins", async () => {
    wallet(3);
    renderPanel(practice);
    expect(await screen.findByText(/2 more coins/)).toBeInTheDocument();
    expect(startButton()).toBeDisabled();
    expect(screen.getByRole("button", { name: "Add coins" })).toBeEnabled();
  });

  it("never assumes the Candidate can pay when the balance fails to load", async () => {
    mocks.get.mockRejectedValue(new AxiosError("Network Error", "ERR_NETWORK"));
    renderPanel(practice);
    expect(
      await screen.findByText("We couldn't load your balance."),
    ).toBeInTheDocument();
    expect(startButton()).toBeDisabled();
  });

  it("keeps the Idempotency-Key across a retry after an ambiguous failure", async () => {
    wallet(9);
    mocks.post
      .mockRejectedValueOnce(new AxiosError("timeout", "ECONNABORTED"))
      .mockResolvedValueOnce({
        status: 200,
        data: { success: true, data: session() },
      });
    renderPanel(practice);
    await screen.findByText("9 coins");
    fireEvent.click(startButton());
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "You won't be charged twice.",
    );
    fireEvent.click(startButton());
    await waitFor(() => expect(mocks.push).toHaveBeenCalled());
    const keys = mocks.post.mock.calls.map(
      ([, , config]) => config.headers["Idempotency-Key"],
    );
    expect(keys[0]).toMatch(uuidPattern);
    expect(keys[1]).toBe(keys[0]);
    // The retry found the session the first attempt made: no claim about coins either way.
    expect(mocks.info).toHaveBeenCalledWith(
      "Opening your existing interview",
      expect.anything(),
    );
  });

  it("starts a new attempt after a failure that proves nothing was created", async () => {
    wallet(9);
    mocks.post
      .mockRejectedValueOnce(apiFailure(409, "QUESTION_BANK_NOT_READY"))
      .mockResolvedValueOnce({
        status: 201,
        data: { success: true, data: session() },
      });
    renderPanel(practice);
    await screen.findByText("9 coins");
    fireEvent.click(startButton());
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Your interview questions are still being prepared.",
    );
    fireEvent.click(startButton());
    await waitFor(() => expect(mocks.push).toHaveBeenCalled());
    const [first, second] = mocks.post.mock.calls.map(
      ([, , config]) => config.headers["Idempotency-Key"],
    );
    expect(second).not.toBe(first);
  });

  it("offers Add coins when the server refuses for balance", async () => {
    wallet(9);
    mocks.post.mockRejectedValue(apiFailure(422, "INSUFFICIENT_BALANCE"));
    renderPanel(practice);
    await screen.findByText("9 coins");
    fireEvent.click(startButton());
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("You need 5 coins");
    expect(
      screen.getByRole("button", { name: "Add coins" }),
    ).toBeInTheDocument();
  });

  it("links a removed avatar back to the interviewer step", async () => {
    wallet(9);
    mocks.post.mockRejectedValue(
      apiFailure(400, "VALIDATION_ERROR", "avatar not found"),
    );
    renderPanel(practice);
    await screen.findByText("9 coins");
    fireEvent.click(startButton());
    expect(
      await screen.findByRole("link", { name: "Change interviewer" }),
    ).toHaveAttribute(
      "href",
      `/interviews/new/interviewer?roleProfileId=${profileId}`,
    );
  });
});

describe("a resume", () => {
  const resume: StartPlan = {
    kind: "resume",
    type: "practice",
    sessionId,
    callout: <p>Unfinished interview</p>,
  };

  it("shows no price, reads no wallet and calls the side-effect-free resume", async () => {
    mocks.post.mockResolvedValue({
      status: 200,
      data: { success: true, data: session({ answered_count: 3 }) },
    });
    renderPanel(resume);
    expect(screen.getByText("Unfinished interview")).toBeInTheDocument();
    expect(screen.queryByText(/coins/)).not.toBeInTheDocument();
    expect(mocks.get).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Resume interview" }));
    await waitFor(() =>
      expect(mocks.push).toHaveBeenCalledWith(`/interviews/${sessionId}/room`),
    );
    expect(mocks.post).toHaveBeenCalledWith(`/interviews/${sessionId}/resume`);
    expect(mocks.post).toHaveBeenCalledTimes(1);
  });

  it("offers a new interview when the session finished in the meantime", async () => {
    mocks.post.mockRejectedValue(apiFailure(409, "INTERVIEW_COMPLETED"));
    renderPanel(resume);
    fireEvent.click(screen.getByRole("button", { name: "Resume interview" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "This interview has already finished.",
    );
    expect(
      screen.getByRole("button", { name: "Set up a new interview" }),
    ).toBeInTheDocument();
  });
});

describe("a recruitment interview", () => {
  const recruitment: StartPlan = {
    kind: "new",
    request: { type: "recruitment", applicationId },
  };

  it("is free, reads no wallet and never names an avatar or voice", async () => {
    mocks.post.mockResolvedValue({
      status: 201,
      data: {
        success: true,
        data: session({
          type: "recruitment",
          role_profile_id: null,
          application_id: applicationId,
        }),
      },
    });
    renderPanel(recruitment);
    expect(
      screen.getByText("Free for you. The company covers this interview."),
    ).toBeInTheDocument();
    expect(screen.queryByText(/coins/)).not.toBeInTheDocument();
    expect(startButton()).toHaveAccessibleName("Start interview");
    fireEvent.click(startButton());
    await waitFor(() => expect(mocks.push).toHaveBeenCalled());
    expect(mocks.get).not.toHaveBeenCalled();
    expect(mocks.post.mock.calls[0][1]).toEqual({
      type: "recruitment",
      application_id: applicationId,
    });
  });

  it("explains a full posting and leads back to the application", async () => {
    mocks.post.mockRejectedValue(apiFailure(409, "INTERVIEW_CAPACITY_FULL"));
    renderPanel(recruitment);
    fireEvent.click(startButton());
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "All interview slots for this posting are taken.",
    );
    expect(
      screen.getByRole("link", { name: "Back to application" }),
    ).toHaveAttribute("href", `/applications/${applicationId}`);
  });
});
