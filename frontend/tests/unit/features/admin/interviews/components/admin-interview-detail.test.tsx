import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AdminInterviewDetail } from "@/features/admin/interviews/components/admin-interview-detail";

const mocks = vi.hoisted(() => ({
  detail: vi.fn(),
  user: { id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", role: "admin" },
}));

vi.mock("@/lib/api/client", () => ({
  apiClient: {
    get: async (url: string) => ({
      data: { success: true, data: await mocks.detail(url) },
    }),
  },
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));

const interviewId = "11111111-1111-4111-8111-111111111111";
const detailUrl = `/admin/interviews/${interviewId}`;

const detail = {
  id: interviewId,
  type: "recruitment",
  owner: {
    id: "22222222-2222-4222-8222-222222222222",
    name: "Alex Candidate",
    email: "alex@example.com",
  },
  job_posting: {
    id: "33333333-3333-4333-8333-333333333333",
    title: "Backend Engineer",
  },
  score: 82,
  has_report: true,
  has_recording: true,
  role_profile: {
    id: "44444444-4444-4444-8444-444444444444",
    title: "Go backend role",
  },
  application_id: "55555555-5555-4555-8555-555555555555",
  avatar: { id: "66666666-6666-4666-8666-666666666666", name: "Ava" },
  voice: null,
  question_count: 5,
  turn_count: 12,
};

function apiError(status: number, code: string) {
  return Object.assign(new Error(code), {
    isAxiosError: true,
    response: {
      status,
      data: { success: false, error: { code, message: code } },
    },
  });
}

function renderDetail(id = interviewId) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <AdminInterviewDetail id={id} />
    </QueryClientProvider>,
  );
}

// The heading is visible while loading, so wait for a definition list row instead.
async function awaitDetailFields() {
  await screen.findByText("Interview ID", { selector: "dt" });
}

function field(label: string) {
  const term = screen.getByText(label, { selector: "dt" });
  return within(term.parentElement as HTMLElement).getByText(
    (_, element) => element?.tagName === "DD",
  );
}

describe("AdminInterviewDetail", () => {
  beforeEach(() => {
    mocks.detail.mockReset();
    mocks.detail.mockResolvedValue(detail);
  });

  it("renders the operational metadata of one interview", async () => {
    renderDetail();

    await awaitDetailFields();
    expect(
      screen.getByRole("heading", { name: "Interview session" }),
    ).toBeVisible();
    expect(mocks.detail).toHaveBeenCalledWith(detailUrl);
    expect(field("Interview ID")).toHaveTextContent(interviewId);
    expect(field("Type")).toHaveTextContent("Recruitment");
    expect(field("Owner")).toHaveTextContent("Alex Candidate");
    expect(field("Owner email")).toHaveTextContent("alex@example.com");
    expect(field("Role Profile")).toHaveTextContent("Go backend role");
    expect(field("Job Posting")).toHaveTextContent("Backend Engineer");
    expect(field("Application ID")).toHaveTextContent(
      "55555555-5555-4555-8555-555555555555",
    );
    expect(field("Avatar")).toHaveTextContent("Ava");
    expect(field("Voice")).toHaveTextContent("None");
    expect(field("Questions")).toHaveTextContent("5");
    expect(field("Turns")).toHaveTextContent("12");
    expect(field("Score")).toHaveTextContent("82");
    expect(field("Report")).toHaveTextContent("Available");
    expect(field("Recording")).toHaveTextContent("Recorded");
  });

  it("is read-only and offers no recording link, transcript or edit action", async () => {
    renderDetail();
    await awaitDetailFields();

    const actions = screen.queryAllByRole("button");
    expect(actions).toHaveLength(0);
    expect(
      screen.queryByRole("link", {
        name: /recording|transcript|audio|video|download/i,
      }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /edit|delete|remove|download/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Back to interview sessions" }),
    ).toHaveAttribute("href", "/admin/interviews");
  });

  it("drops any content field the backend might send", async () => {
    mocks.detail.mockResolvedValue({
      ...detail,
      transcript: "SECRET TRANSCRIPT TEXT",
      record_path: "recordings/secret.webm",
      answer: "SECRET ANSWER",
    });
    const { container } = renderDetail();
    await awaitDetailFields();

    expect(container).not.toHaveTextContent("SECRET TRANSCRIPT TEXT");
    expect(container).not.toHaveTextContent("recordings/secret.webm");
    expect(container).not.toHaveTextContent("SECRET ANSWER");
  });

  it("shows practice interviews without a posting or application", async () => {
    mocks.detail.mockResolvedValue({
      ...detail,
      type: "practice",
      job_posting: null,
      application_id: null,
      avatar: null,
      score: null,
      has_report: false,
      has_recording: false,
    });
    renderDetail();
    await awaitDetailFields();

    expect(field("Type")).toHaveTextContent("Practice");
    expect(field("Job Posting")).toHaveTextContent("None (practice interview)");
    expect(field("Application ID")).toHaveTextContent(
      "None (practice interview)",
    );
    expect(field("Score")).toHaveTextContent("Not scored");
    expect(field("Report")).toHaveTextContent("Not available");
    expect(field("Recording")).toHaveTextContent("Not recorded");
  });

  it("shows a not-found state for an unknown interview", async () => {
    mocks.detail.mockRejectedValue(apiError(404, "INTERVIEW_NOT_FOUND"));
    renderDetail();

    expect(
      await screen.findByRole("heading", { name: "Interview not found" }),
    ).toBeVisible();
  });

  it("does not call the API for a malformed id", async () => {
    renderDetail("not-a-uuid");

    expect(
      await screen.findByRole("heading", { name: "Interview not found" }),
    ).toBeVisible();
    expect(mocks.detail).not.toHaveBeenCalled();
  });

  it("shows the access check when admin access is refused", async () => {
    mocks.detail.mockRejectedValue(apiError(403, "FORBIDDEN"));
    renderDetail();

    expect(
      await screen.findByText(/We couldn.t confirm your administrator access/),
    ).toBeVisible();
    expect(
      screen.queryByRole("heading", { name: "Interview session" }),
    ).toBeVisible();
    expect(screen.queryByText("Backend Engineer")).not.toBeInTheDocument();
  });

  it("shows a retryable error for other failures", async () => {
    mocks.detail.mockRejectedValue(apiError(500, "INTERNAL"));
    renderDetail();

    expect(
      await screen.findByText(
        "Unable to load this interview session. Please try again.",
      ),
    ).toBeVisible();
    expect(screen.getByRole("button", { name: "Retry" })).toBeVisible();
  });
});
