import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { InterviewResult } from "@/features/recruiter/applications/components/interview-result";
import type { ApplicationInterview } from "@/features/recruiter/applications/schemas/application-detail-schema";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  user: {
    id: "11111111-1111-4111-8111-111111111111",
    full_name: "Rec",
    email: "rec@example.com",
    role: "recruiter" as const,
    email_verified: true,
    is_locked: false,
    image: null,
  },
}));
vi.mock("@/lib/api/client", () => ({ apiClient: { get: mocks.get } }));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));

const postingId = "33333333-3333-4333-8333-333333333333";
const applicationId = "44444444-4444-4444-8444-444444444444";
const interviewId = "66666666-6666-4666-8666-666666666666";

const completed: ApplicationInterview = {
  id: interviewId,
  started_at: "2026-10-20T08:00:00Z",
  completed_at: "2026-10-20T08:30:00Z",
  status: "completed",
  score: 82,
  record_status: "ready",
};

const report = {
  success: true,
  data: {
    interview_id: interviewId,
    type: "recruitment",
    status: "completed",
    overall_score: 82,
    started_at: "2026-10-20T08:00:00Z",
    completed_at: "2026-10-20T08:30:00Z",
    summary: "Solid Go answers, weaker on testing.",
    breakdown: [
      {
        metric_id: "77777777-7777-4777-8777-777777777777",
        name: "Go depth",
        score: 90,
      },
      {
        metric_id: "88888888-8888-4888-8888-888888888888",
        name: "Communication",
        score: 74,
      },
    ],
    questions: null,
  },
};
const recording = {
  success: true,
  data: {
    record_status: "ready",
    url: "https://files.example.com/rec/linh.webm?signature=test",
    expires_at: "2026-10-24T10:05:00Z",
  },
};
const transcript = {
  success: true,
  data: {
    turns: [
      {
        position: 1,
        question: "Tell us about a service you built.",
        answer: "I built a queue worker.",
      },
      { position: 2, question: "How do you test it?", answer: null },
    ],
  },
};

function failure(status: number) {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: {
      success: false,
      error: { code: "INTERNAL_ERROR", message: "boom" },
    },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}

function routes(path: string) {
  if (path.endsWith("/report")) return Promise.resolve({ data: report });
  if (path.endsWith("/recording")) return Promise.resolve({ data: recording });
  if (path.endsWith("/transcript"))
    return Promise.resolve({ data: transcript });
  return Promise.reject(new Error(`unexpected GET ${path}`));
}

function mount(interview: ApplicationInterview = completed) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  renderWithIntl(
    <QueryClientProvider client={client}>
      <InterviewResult
        postingId={postingId}
        applicationId={applicationId}
        interview={interview}
      />
    </QueryClientProvider>,
  );
  return client;
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
  mocks.get.mockImplementation(routes);
});

describe("InterviewResult", () => {
  it("shows the score and status without fetching any evidence", () => {
    mount();
    expect(screen.getByText("82/100")).toBeInTheDocument();
    expect(screen.getByText("Evaluated")).toBeInTheDocument();
    expect(mocks.get).not.toHaveBeenCalled();
  });

  it("says the interview is unfinished and offers no evaluation while in progress", () => {
    mount({
      ...completed,
      completed_at: null,
      status: "in_progress",
      score: null,
    });
    expect(screen.getByText("Not scored yet")).toBeInTheDocument();
    expect(
      screen.getByText("The Candidate has not finished the interview yet."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Show evaluation" }),
    ).not.toBeInTheDocument();
  });

  it("loads the evaluation breakdown only when the Recruiter asks", async () => {
    mount();
    expect(mocks.get).not.toHaveBeenCalledWith(
      `/interviews/${interviewId}/report`,
      expect.anything(),
    );

    fireEvent.click(screen.getByRole("button", { name: "Show evaluation" }));

    expect(
      await screen.findByText("Solid Go answers, weaker on testing."),
    ).toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledWith(
      `/interviews/${interviewId}/report`,
      {
        signal: expect.any(AbortSignal),
      },
    );
    expect(screen.getByText("Go depth")).toBeInTheDocument();
    expect(screen.getByText("90/100")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Hide evaluation" }),
    ).toHaveAttribute("aria-expanded", "true");
  });

  it("shows an evaluation error with a way to retry", async () => {
    mocks.get.mockRejectedValueOnce(failure(500));
    mount();
    fireEvent.click(screen.getByRole("button", { name: "Show evaluation" }));

    expect(
      await screen.findByText("We couldn't load the evaluation. Try again."),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(
      await screen.findByText("Solid Go answers, weaker on testing."),
    ).toBeInTheDocument();
  });

  it("requests the recording link only on demand", async () => {
    mount();
    expect(screen.getByText("A recording is available.")).toBeInTheDocument();
    expect(mocks.get).not.toHaveBeenCalledWith(
      expect.stringMatching(/recording$/),
      expect.anything(),
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Get the recording link" }),
    );

    expect(
      await screen.findByRole("link", { name: /Open the recording/ }),
    ).toHaveAttribute(
      "href",
      "https://files.example.com/rec/linh.webm?signature=test",
    );
  });

  it("shows a recording that is still processing without a link", () => {
    mount({ ...completed, record_status: "processing" });
    expect(
      screen.getByText("The recording is still being processed."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Get the recording link" }),
    ).not.toBeInTheDocument();
  });

  it("loads the transcript only when the Recruiter opens it", async () => {
    mount();
    expect(
      screen.queryByText("Tell us about a service you built."),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Show transcript" }));

    expect(
      await screen.findByText("Tell us about a service you built."),
    ).toBeInTheDocument();
    expect(screen.getByText("I built a queue worker.")).toBeInTheDocument();
    expect(screen.getByText("No answer was recorded.")).toBeInTheDocument();
  });

  it("offers no transcript until the interview is completed", () => {
    mount({ ...completed, completed_at: null, status: "in_progress" });
    expect(
      screen.getByText(
        "The transcript is available once the interview is completed.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Show transcript" }),
    ).not.toBeInTheDocument();
  });
});
