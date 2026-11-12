import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { renderWithIntl } from "@tests/helpers/intl";
import { PostingReview } from "@/features/admin/moderation/components/posting-review";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  user: { id: "11111111-1111-4111-8111-111111111111", role: "admin" as const },
}));
vi.mock("@/lib/api/client", () => ({
  apiClient: { get: mocks.get, post: mocks.post },
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));
vi.mock("goey-toast", () => ({ gooeyToast: { success: vi.fn() } }));

const postingId = "33333333-3333-4333-8333-333333333333";
const failingCoverage = {
  passes: false,
  assessed: true,
  covered: 4,
  total: 5,
  question_limit: 8,
  missing_metrics: [
    {
      id: "88888888-8888-4888-8888-888888888881",
      name: "Depth of Understanding",
      suggested_rubrics: [
        {
          id: "99999999-9999-4999-8999-999999999991",
          name: "Explain a mechanism",
        },
      ],
    },
  ],
  exceeds_limit: false,
  min_questions: 0,
};
const posting = (coverage: unknown) => ({
  id: postingId,
  title: "Go Backend Engineer",
  status: "pending",
  reject_reason: null,
  recruiter: {
    id: "22222222-2222-4222-8222-222222222222",
    name: "Rec Ruiter",
    email: "rec@example.com",
  },
  location: null,
  employment_type: null,
  salary_min: null,
  salary_max: null,
  interview_slot: null,
  interview_duration: null,
  required_avatar: null,
  required_voice: null,
  created_at: "2026-10-12T08:00:00Z",
  content_path: "https://files.example.com/jd/go-backend.pdf",
  document_url: null,
  document_expires_at: null,
  raw_text: null,
  review_submitted: true,
  core_question_count: 8,
  coverage,
});
const envelope = (data: unknown) => ({ data: { success: true, data } });

function mount(locale: "en" | "vi" = "en") {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  renderWithIntl(
    <QueryClientProvider client={client}>
      <PostingReview id={postingId} backQuery="" />
    </QueryClientProvider>,
    { locale },
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  for (const fn of [mocks.get, mocks.post]) fn.mockReset();
});

describe("Admin review and metric coverage", () => {
  it("names the missing metric, keeps Approve off with the reason, and leaves Reject available", async () => {
    mocks.get.mockResolvedValue(envelope(posting(failingCoverage)));
    mount();

    expect(
      await screen.findByRole("heading", {
        name: "Metric coverage",
      }),
    ).toBeInTheDocument();
    expect(screen.getByRole("meter")).toHaveAttribute("aria-valuenow", "4");
    expect(screen.getByText("Not measured:")).toBeInTheDocument();
    expect(
      screen.getByText(
        "The Recruiter must fix the question bank before approval.",
      ),
    ).toBeInTheDocument();
    // The Recruiter's rubric hints are not for the Admin.
    expect(screen.queryByText(/for example/)).not.toBeInTheDocument();
    const approve = screen.getByRole("button", { name: "Approve" });
    expect(approve).toBeDisabled();
    expect(
      screen.getByText("The question bank doesn't cover every metric yet."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Reject" })).toBeEnabled();
  });

  it("lets the Admin approve a bank that covers every metric", async () => {
    mocks.get.mockResolvedValue(
      envelope(
        posting({
          ...failingCoverage,
          passes: true,
          covered: 5,
          missing_metrics: [],
        }),
      ),
    );
    mount();

    expect(
      await screen.findByRole("button", { name: "Approve" }),
    ).toBeEnabled();
    expect(screen.getByText("All metrics covered")).toBeInTheDocument();
  });

  it("shows the translated block reason when the server refuses an approval the page thought possible", async () => {
    mocks.get.mockResolvedValue(
      envelope(
        posting({
          ...failingCoverage,
          passes: true,
          covered: 5,
          missing_metrics: [],
        }),
      ),
    );
    mocks.post.mockRejectedValue(
      new AxiosError("refused", "ERR_BAD_REQUEST", undefined, undefined, {
        status: 409,
        statusText: "Conflict",
        data: {
          success: false,
          error: {
            code: "METRIC_COVERAGE_INCOMPLETE",
            message: "backend english",
          },
        },
        headers: new AxiosHeaders(),
        config: { headers: new AxiosHeaders() },
      }),
    );
    mount();

    fireEvent.click(await screen.findByRole("button", { name: "Approve" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Approve posting" }),
    );

    expect(
      await screen.findByText(
        "This posting's questions don't cover every evaluation metric yet. Fix the coverage and try again.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("backend english")).not.toBeInTheDocument();
    // The page rereads the dossier to show the truth.
    await waitFor(() =>
      expect(mocks.get.mock.calls.length).toBeGreaterThanOrEqual(2),
    );
  });

  it("reads in Vietnamese", async () => {
    mocks.get.mockResolvedValue(envelope(posting(failingCoverage)));
    mount("vi");

    expect(
      await screen.findByRole("heading", {
        name: "Độ phủ tiêu chí",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Bộ câu hỏi chưa phủ hết các tiêu chí."),
    ).toBeInTheDocument();
  });
});
