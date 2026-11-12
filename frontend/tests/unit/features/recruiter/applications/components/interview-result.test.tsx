import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, within } from "@testing-library/react";
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
  score: 62.5,
  score_outcome: "partial",
  record_status: "ready",
  end_reason: "completed",
};

const goDepth = "77777777-7777-4777-8777-777777777777";
const communication = "88888888-8888-4888-8888-888888888888";

// The spec's worked example (KAN-337): two graded questions, metrics 70 and 50 weighted 60/40, overall 62.
const report = {
  success: true,
  data: {
    interview_id: interviewId,
    type: "recruitment",
    status: "completed",
    overall_score: 62,
    started_at: "2026-10-20T08:00:00Z",
    completed_at: "2026-10-20T08:30:00Z",
    summary: "Solid Go answers, weaker on testing.",
    scoring: {
      method: "rubric",
      outcome: "complete",
      withheld_reason: null,
      assigned_questions: 2,
      graded_questions: 2,
      weight_total: 100,
      decimals: 2,
    },
    breakdown: [
      {
        metric_id: goDepth,
        name: "Go depth",
        score: 70,
        weight: 60,
        question_count: 2,
      },
      {
        metric_id: communication,
        name: "Communication",
        score: 50,
        weight: 40,
        question_count: 2,
      },
    ],
    questions: [
      {
        position: 1,
        question_number: 1,
        kind: "core",
        question: "Tell us about a service you built.",
        answer: "I built a queue worker.",
        feedback: null,
      },
    ],
    graded_questions: [
      {
        question_number: 1,
        status: "graded",
        rubric_name: "Explain a mechanism",
        total: 70,
        earned: 7,
        max: 10,
        metrics: [
          {
            metric_id: goDepth,
            name: "Go depth",
            earned: 4,
            max: 5,
            score: 80,
          },
          {
            metric_id: communication,
            name: "Communication",
            earned: 3,
            max: 5,
            score: 60,
          },
        ],
        criteria: [
          {
            criterion_id: "99999999-9999-4999-8999-999999999991",
            name: "Explains the core mechanism",
            metric_id: goDepth,
            metric_name: "Go depth",
            points: 2,
            max_points: 3,
            evidence: "I built a queue worker.",
            evidence_position: 1,
            feedback: "Names the worker but not the retry policy.",
          },
        ],
      },
      {
        question_number: 2,
        status: "not_answered",
        rubric_name: null,
        total: null,
        earned: 0,
        max: 0,
        metrics: [],
        criteria: [],
      },
    ],
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
        question_number: 1,
        kind: "core",
        question: "Tell us about a service you built.",
        answer: "I built a queue worker.",
      },
      {
        position: 2,
        question_number: 1,
        kind: "follow_up",
        question: "How does it retry a failed job?",
        answer: "With backoff.",
      },
      {
        position: 3,
        question_number: 2,
        kind: "core",
        question: "How do you test it?",
        answer: null,
      },
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
    expect(screen.getByText("62.50/100")).toBeInTheDocument();
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
    // The overall, the formula with the stored values, the weighted metrics and each criterion.
    expect(screen.getByTestId("overall-score")).toHaveTextContent(
      "62.00 / 100",
    );
    expect(screen.getByTestId("overall-formula")).toHaveTextContent(
      "(70.00 × 60 + 50.00 × 40) ÷ 100 = 62.00",
    );
    const metrics = screen.getByTestId("metric-list");
    expect(within(metrics).getByText("Go depth")).toBeInTheDocument();
    expect(metrics).toHaveTextContent("Weight 60.0%");
    expect(
      screen.getByText(/Question totals are for reference only/),
    ).toBeInTheDocument();
    const grading = screen.getByTestId("question-grading");
    expect(grading).toHaveTextContent("7 / 10 points");
    expect(grading).toHaveTextContent("Question total 70.00 (reference)");
    expect(
      within(grading).getByText("Explains the core mechanism"),
    ).toBeInTheDocument();
    expect(grading).toHaveTextContent("2 / 3 points");
    expect(
      screen.getByText("Not answered: not graded and not counted as zero."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Hide evaluation" }),
    ).toHaveAttribute("aria-expanded", "true");
  });

  it("labels the summary AI-generated, flags the evaluation and quotes the evidence", async () => {
    mocks.get.mockImplementation((path: string) =>
      path.endsWith("/report")
        ? Promise.resolve({
            data: {
              ...report,
              data: {
                ...report.data,
                summary_source: "ai",
                injection_flagged: true,
              },
            },
          })
        : routes(path),
    );
    mount();
    fireEvent.click(screen.getByRole("button", { name: "Show evaluation" }));

    expect(await screen.findByText("AI-generated summary")).toBeInTheDocument();
    expect(screen.getByTestId("injection-flagged")).toHaveTextContent(
      "Flagged for review",
    );
    // The criterion quotes the Candidate's answer.
    expect(
      within(screen.getByTestId("criterion")).getByText(
        "I built a queue worker.",
      ),
    ).toBeInTheDocument();
  });

  it("explains a withheld overall without inventing a score", async () => {
    mocks.get.mockImplementation((path: string) =>
      path.endsWith("/report")
        ? Promise.resolve({
            data: {
              ...report,
              data: {
                ...report.data,
                overall_score: null,
                scoring: {
                  ...report.data.scoring,
                  outcome: "withheld",
                  withheld_reason: "missing_metric_coverage",
                },
                breakdown: [
                  report.data.breakdown[0],
                  {
                    ...report.data.breakdown[1],
                    score: null,
                    question_count: 0,
                  },
                ],
              },
            },
          })
        : routes(path),
    );
    mount({ ...completed, score: null, score_outcome: "withheld" });
    expect(screen.getByText("No overall score")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Show evaluation" }));
    expect(await screen.findByTestId("overall-withheld")).toHaveTextContent(
      "No overall score",
    );
    expect(
      screen.getByText(/No graded question assessed Communication/),
    ).toBeInTheDocument();
    expect(screen.getByTestId("scoring-outcome")).toHaveTextContent("Withheld");
    expect(screen.queryByTestId("overall-formula")).not.toBeInTheDocument();
    expect(screen.getByTestId("metric-list")).toHaveTextContent("No evidence");
  });

  it("shows no flag banner for an unflagged evaluation", async () => {
    mount();
    fireEvent.click(screen.getByRole("button", { name: "Show evaluation" }));
    await screen.findByText("Solid Go answers, weaker on testing.");
    expect(screen.queryByTestId("injection-flagged")).not.toBeInTheDocument();
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

  it("says how the interview ended and that a purged recording was deleted", () => {
    mount({ ...completed, end_reason: "left", record_status: "deleted" });
    expect(screen.getByText("How it ended")).toBeInTheDocument();
    expect(screen.getByText("The Candidate left early")).toBeInTheDocument();
    expect(
      screen.getByText("The recording was deleted after the retention period."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Get the recording link" }),
    ).not.toBeInTheDocument();
  });

  it("shows no end reason for an interview ended before it was recorded", () => {
    mount({ ...completed, end_reason: null });
    expect(screen.queryByText("How it ended")).not.toBeInTheDocument();
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
    // The follow-up sits under its core question; numbering counts core questions.
    const first = screen
      .getByText("Tell us about a service you built.")
      .closest("li") as HTMLElement;
    expect(
      within(first).getByText("How does it retry a failed job?"),
    ).toBeInTheDocument();
    expect(within(first).getByText("Follow-up")).toBeInTheDocument();
    const second = screen
      .getByText("How do you test it?")
      .closest("li") as HTMLElement;
    expect(within(second).getByText("Question 2")).toBeInTheDocument();
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
