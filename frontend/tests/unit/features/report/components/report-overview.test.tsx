import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import {
  REPORT_MAX_POLLS,
  REPORT_POLL_MS,
  ReportOverview,
} from "@/features/report/components/report-overview";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  user: {
    id: "11111111-1111-4111-8111-111111111111",
    full_name: "Cand",
    email: "cand@example.com",
    role: "candidate" as const,
    email_verified: true,
    is_locked: false,
    image: null,
  },
}));
vi.mock("@/lib/api/client", () => ({ apiClient: { get: mocks.get } }));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));

const INTERVIEW = "22222222-2222-4222-8222-222222222222";
const METRIC_A = "55555555-5555-4555-8555-555555555551";
const METRIC_B = "55555555-5555-4555-8555-555555555552";

const report = (over: Record<string, unknown> = {}) => ({
  interview_id: INTERVIEW,
  type: "practice",
  status: "completed",
  overall_score: 72,
  started_at: "2026-10-20T08:00:00Z",
  completed_at: "2026-10-20T08:30:00Z",
  summary: "Clear structure, thin examples.",
  breakdown: [
    { metric_id: METRIC_A, name: "Communication", score: 80 },
    { metric_id: METRIC_B, name: "Depth", score: 64 },
  ],
  questions: [
    {
      position: 1,
      question_number: 1,
      kind: "core",
      question: "Tell me about a project.",
      answer: "I built an API.",
      feedback: "Name the impact.",
    },
    {
      position: 2,
      question_number: 2,
      kind: "core",
      question: "Why this role?",
      answer: null,
      feedback: null,
    },
  ],
  ...over,
});

const envelope = (data: unknown) => ({ data: { success: true, data } });

function failure(status: number, code = "INTERNAL_ERROR") {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: { success: false, error: { code, message: "server says" } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}

function renderReport(id = INTERVIEW) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  return renderWithIntl(
    <QueryClientProvider client={client}>
      <ReportOverview reportId={id} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
});

describe("ReportOverview", () => {
  it("shows a loading state, then the completed practice report", async () => {
    mocks.get.mockResolvedValue(envelope(report()));
    renderReport();
    expect(screen.getByRole("status")).toHaveTextContent(
      /loading your report/i,
    );

    expect(
      await screen.findByRole("heading", { name: "Practice interview" }),
    ).toBeInTheDocument();
    expect(screen.getByText("72 / 100")).toBeInTheDocument();
    expect(
      screen.getByText("Clear structure, thin examples."),
    ).toBeInTheDocument();

    const communication = screen.getByRole("progressbar", {
      name: "Communication",
    });
    expect(communication).toHaveAttribute("aria-valuenow", "80");
    expect(screen.getByRole("progressbar", { name: "Depth" })).toHaveAttribute(
      "aria-valuenow",
      "64",
    );

    expect(screen.getByText("Tell me about a project.")).toBeInTheDocument();
    expect(screen.getByText("I built an API.")).toBeInTheDocument();
    expect(screen.getByText("Name the impact.")).toBeInTheDocument();
    expect(screen.getByText("No answer recorded.")).toBeInTheDocument();

    expect(mocks.get).toHaveBeenCalledWith(`/interviews/${INTERVIEW}/report`, {
      signal: expect.any(AbortSignal),
    });
  });

  it("says the Candidate left early and counts the unanswered questions", async () => {
    const answered = {
      position: 1,
      question_number: 1,
      kind: "core",
      question: "Q1",
      answer: "A1",
      feedback: null,
    };
    mocks.get.mockResolvedValue(
      envelope(
        report({
          end_reason: "left",
          question_count: 8,
          questions: [
            answered,
            { ...answered, position: 2, question_number: 2 },
          ],
        }),
      ),
    );
    renderReport();
    const notice = await screen.findByTestId("report-end-notice");
    expect(
      within(notice).getByRole("heading", {
        name: "You left the interview early",
      }),
    ).toBeInTheDocument();
    expect(notice).toHaveTextContent(
      "2 of 8 questions answered; 6 questions not answered.",
    );
  });

  it("explains an abandoned interview and shows nothing for a normal completion", async () => {
    mocks.get.mockResolvedValue(
      envelope(report({ end_reason: "abandoned", question_count: 2 })),
    );
    const view = renderReport();
    expect(
      await screen.findByText("This interview ended because it was inactive"),
    ).toBeInTheDocument();
    view.unmount();

    mocks.get.mockResolvedValue(envelope(report({ end_reason: "completed" })));
    renderReport();
    await screen.findByRole("heading", { name: "Practice interview" });
    expect(screen.queryByTestId("report-end-notice")).not.toBeInTheDocument();
  });

  it("shows a follow-up under its core question, numbered by core questions", async () => {
    mocks.get.mockResolvedValue(
      envelope(
        report({
          questions: [
            {
              position: 1,
              question_number: 1,
              kind: "core",
              question: "How would you design a rate limiter?",
              answer: "A token bucket.",
              feedback: "Good start.",
            },
            {
              position: 2,
              question_number: 1,
              kind: "follow_up",
              question: "How do bursts refill?",
              answer: "At a fixed rate.",
              feedback: "Clear refill rule.",
            },
            {
              position: 3,
              question_number: 2,
              kind: "core",
              question: "How do you test it?",
              answer: null,
              feedback: null,
            },
          ],
        }),
      ),
    );
    renderReport();
    const core = (
      await screen.findByText("How would you design a rate limiter?")
    ).closest("li") as HTMLElement;
    const followUp = within(core).getByText("How do bursts refill?");
    expect(within(core).getByText("Follow-up")).toBeInTheDocument();
    expect(within(core).getByText("Clear refill rule.")).toBeInTheDocument();
    expect(followUp.closest("li")).not.toBe(core);
    const second = screen
      .getByText("How do you test it?")
      .closest("li") as HTMLElement;
    expect(within(second).getByText("2.")).toBeInTheDocument();
    expect(within(second).queryByText("Follow-up")).not.toBeInTheDocument();
  });

  it("renders metric names exactly as the API sends them", async () => {
    mocks.get.mockResolvedValue(
      envelope(
        report({
          breakdown: [{ metric_id: METRIC_A, name: "Ownership", score: 10 }],
        }),
      ),
    );
    renderReport();
    const bars = await screen.findAllByRole("progressbar");
    expect(bars).toHaveLength(1);
    expect(
      within(bars[0].parentElement as HTMLElement).getByText("Ownership"),
    ).toBeInTheDocument();
  });

  it("keeps the in-progress interview honest, with a check-again action", async () => {
    mocks.get.mockResolvedValue(
      envelope(
        report({
          status: "in_progress",
          overall_score: null,
          completed_at: null,
          summary: null,
          breakdown: null,
          questions: null,
        }),
      ),
    );
    renderReport();
    expect(
      await screen.findByRole("heading", {
        name: "This interview isn't finished yet",
      }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/overall score/i)).not.toBeInTheDocument();
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Check again" }));
    await waitFor(() => expect(mocks.get).toHaveBeenCalledTimes(2));
  });

  it("shows evaluating state while the scores are not ready", async () => {
    mocks.get.mockResolvedValue(
      envelope(
        report({
          status: "pending",
          overall_score: null,
          summary: null,
          breakdown: null,
          questions: null,
        }),
      ),
    );
    renderReport();
    expect(
      await screen.findByRole("heading", {
        name: "Your interview is being evaluated",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Scores and feedback will appear here when the evaluation is done.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Check again" })).toBeEnabled();
  });

  it("reports a failed evaluation without showing any score", async () => {
    mocks.get.mockResolvedValue(
      envelope(
        report({
          status: "failed",
          overall_score: null,
          summary: null,
          breakdown: null,
          questions: null,
        }),
      ),
    );
    renderReport();
    expect(
      await screen.findByRole("heading", {
        name: "We couldn't evaluate this interview",
      }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Overall score")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Check again" }),
    ).not.toBeInTheDocument();
  });

  it("shows only the score for a recruitment report the Candidate may see", async () => {
    mocks.get.mockResolvedValue(
      envelope(
        report({
          type: "recruitment",
          summary: null,
          breakdown: null,
          questions: null,
        }),
      ),
    );
    renderReport();
    expect(
      await screen.findByRole("heading", { name: "Recruitment interview" }),
    ).toBeInTheDocument();
    expect(screen.getByText("72 / 100")).toBeInTheDocument();
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
    expect(
      screen.getByText(
        "Detailed feedback for recruitment interviews is not shown here.",
      ),
    ).toBeInTheDocument();
  });

  it("shows a not-found state for a report the Candidate cannot read", async () => {
    mocks.get.mockRejectedValue(failure(404, "INTERVIEW_NOT_FOUND"));
    renderReport();
    expect(
      await screen.findByRole("heading", {
        name: "We couldn't find this report",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Open interview history" }),
    ).toHaveAttribute("href", "/history");
  });

  it("explains a refused request as an access problem", async () => {
    mocks.get.mockRejectedValue(failure(403, "FORBIDDEN"));
    renderReport();
    expect(
      await screen.findByRole("heading", {
        name: "You don't have access to this report",
      }),
    ).toBeInTheDocument();
  });

  it("offers a retry when the report cannot load", async () => {
    mocks.get
      .mockRejectedValueOnce(failure(500))
      .mockResolvedValueOnce(envelope(report()));
    renderReport();
    expect(
      await screen.findByRole("heading", {
        name: "We couldn't load this report",
      }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(
      await screen.findByRole("heading", { name: "Practice interview" }),
    ).toBeInTheDocument();
  });

  it("rejects an invalid link without calling the API", () => {
    renderReport("not-a-uuid");
    expect(
      screen.getByRole("heading", { name: "This link isn't valid" }),
    ).toBeInTheDocument();
    expect(mocks.get).not.toHaveBeenCalled();
  });

  describe("while evaluation is pending", () => {
    const pending = envelope(
      report({
        status: "pending",
        overall_score: null,
        summary: null,
        breakdown: null,
        questions: null,
      }),
    );
    beforeEach(() => {
      vi.useFakeTimers({ toFake: ["setInterval", "clearInterval", "Date"] });
    });
    afterEach(() => vi.useRealTimers());

    it("refreshes by itself and stops once the report is completed", async () => {
      mocks.get
        .mockResolvedValueOnce(pending)
        .mockResolvedValueOnce(pending)
        .mockResolvedValue(envelope(report()));
      renderReport();
      expect(
        await screen.findByText("We'll update this page automatically."),
      ).toBeInTheDocument();
      await act(() => vi.advanceTimersByTimeAsync(REPORT_POLL_MS));
      await waitFor(() => expect(mocks.get).toHaveBeenCalledTimes(2));
      await act(() => vi.advanceTimersByTimeAsync(REPORT_POLL_MS));
      expect(
        await screen.findByRole("heading", { name: "Overall score" }),
      ).toBeInTheDocument();
      await act(() => vi.advanceTimersByTimeAsync(REPORT_POLL_MS * 3));
      expect(mocks.get).toHaveBeenCalledTimes(3);
    });

    it("gives up after a bounded number of checks and leaves Check again", async () => {
      mocks.get.mockResolvedValue(pending);
      renderReport();
      await screen.findByText("We'll update this page automatically.");
      for (let poll = 1; poll < REPORT_MAX_POLLS + 5; poll++)
        await act(() => vi.advanceTimersByTimeAsync(REPORT_POLL_MS));
      expect(mocks.get).toHaveBeenCalledTimes(REPORT_MAX_POLLS);
      expect(
        await screen.findByText("Still evaluating. Check again in a moment."),
      ).toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: "Check again" }));
      await waitFor(() =>
        expect(mocks.get).toHaveBeenCalledTimes(REPORT_MAX_POLLS + 1),
      );
    });

    it("stays bounded when every refresh fails", async () => {
      mocks.get.mockResolvedValueOnce(pending).mockRejectedValue(failure(500));
      renderReport();
      await screen.findByText("We'll update this page automatically.");
      for (let poll = 1; poll < REPORT_MAX_POLLS + 5; poll++)
        await act(() => vi.advanceTimersByTimeAsync(REPORT_POLL_MS));
      expect(mocks.get).toHaveBeenCalledTimes(REPORT_MAX_POLLS);
    });

    it("does not poll a failed evaluation", async () => {
      mocks.get.mockResolvedValue(
        envelope(report({ status: "failed", overall_score: null })),
      );
      renderReport();
      await screen.findByText("Practice interview");
      await act(() => vi.advanceTimersByTimeAsync(REPORT_POLL_MS * 3));
      expect(mocks.get).toHaveBeenCalledTimes(1);
    });
  });
});
