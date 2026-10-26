import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AdminInterviewSessions } from "@/features/admin/interviews/components/admin-interview-sessions";

const mocks = vi.hoisted(() => ({
  list: vi.fn(),
  user: { id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", role: "admin" },
}));

vi.mock("@/lib/api/client", () => ({
  apiClient: {
    get: async (_url: string, options: { params?: unknown }) => ({
      data: { success: true, data: await mocks.list(options.params) },
    }),
  },
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));

// Radix Select calls scrollIntoView when it opens; jsdom does not implement it.
if (!HTMLElement.prototype.scrollIntoView)
  Object.defineProperty(HTMLElement.prototype, "scrollIntoView", {
    configurable: true,
    value: () => {},
  });

const interviewId = "11111111-1111-4111-8111-111111111111";
const candidateId = "22222222-2222-4222-8222-222222222222";
const postingId = "33333333-3333-4333-8333-333333333333";

const recruitmentRow = {
  id: interviewId,
  type: "recruitment",
  owner: {
    id: candidateId,
    name: "Alex Candidate",
    email: "alex@example.com",
  },
  job_posting: { id: postingId, title: "Backend Engineer" },
  score: 82,
  has_report: true,
  has_recording: true,
};
const practiceRow = {
  id: "44444444-4444-4444-8444-444444444444",
  type: "practice",
  owner: {
    id: "55555555-5555-4555-8555-555555555555",
    name: "",
    email: "sam@example.com",
  },
  job_posting: null,
  score: null,
  has_report: false,
  has_recording: false,
};

function page(
  items: unknown[] = [recruitmentRow, practiceRow],
  total = items.length,
  pageNumber = 1,
) {
  return { items, total, page: pageNumber, page_size: 20 };
}

function apiError(status: number, code: string) {
  return Object.assign(new Error(code), {
    isAxiosError: true,
    response: {
      status,
      data: { success: false, error: { code, message: code } },
    },
  });
}

function renderScreen() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <AdminInterviewSessions />
    </QueryClientProvider>,
  );
}

async function chooseOption(label: string, name: string) {
  fireEvent.keyDown(screen.getByRole("combobox", { name: label }), {
    key: "ArrowDown",
  });
  fireEvent.keyDown(await screen.findByRole("option", { name }), {
    key: "Enter",
  });
}

describe("AdminInterviewSessions", () => {
  beforeEach(() => {
    mocks.list.mockReset();
    mocks.list.mockResolvedValue(page());
  });

  it("lists operational metadata for each interview with a detail link", async () => {
    renderScreen();

    const table = await screen.findByRole("table", {
      name: "Interview sessions",
    });
    const recruitment = within(table).getByRole("row", {
      name: /Alex Candidate/,
    });
    expect(within(recruitment).getByText("Backend Engineer")).toBeVisible();
    expect(within(recruitment).getByText("82")).toBeVisible();
    expect(within(recruitment).getByText("Available")).toBeVisible();
    expect(within(recruitment).getByText("Recorded")).toBeVisible();
    expect(
      within(recruitment).getByRole("link", {
        name: `View details for interview ${interviewId}`,
      }),
    ).toHaveAttribute("href", `/admin/interviews/${interviewId}`);

    const practice = within(table).getByRole("row", {
      name: /sam@example.com/,
    });
    expect(within(practice).getByText("Practice (no posting)")).toBeVisible();
    expect(within(practice).getByText("Not scored")).toBeVisible();
    expect(within(practice).getByText("Not recorded")).toBeVisible();
    expect(screen.getByText("2 interview sessions")).toBeVisible();
  });

  it("never offers a recording, transcript, edit or delete action", async () => {
    renderScreen();
    await screen.findByRole("table", { name: "Interview sessions" });

    expect(
      screen.queryByRole("button", { name: /edit|delete|remove|download/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: /recording|transcript|audio|video/i }),
    ).not.toBeInTheDocument();
  });

  it("requests the list with the type, candidate and Job Posting filters", async () => {
    renderScreen();
    await screen.findByRole("table", { name: "Interview sessions" });

    await chooseOption("Interview type", "Recruitment");
    fireEvent.change(screen.getByLabelText("Candidate ID"), {
      target: { value: candidateId },
    });
    fireEvent.change(screen.getByLabelText("Job Posting ID"), {
      target: { value: postingId },
    });
    fireEvent.click(screen.getByRole("button", { name: "Apply filters" }));

    await waitFor(() =>
      expect(mocks.list).toHaveBeenLastCalledWith({
        page: 1,
        page_size: 20,
        type: "recruitment",
        candidate_id: candidateId,
        job_posting_id: postingId,
      }),
    );
    expect(
      await screen.findByText("2 matching interview sessions"),
    ).toBeVisible();
  });

  it("blocks a malformed candidate ID before any request", async () => {
    renderScreen();
    await screen.findByRole("table", { name: "Interview sessions" });
    const callsBefore = mocks.list.mock.calls.length;

    fireEvent.change(screen.getByLabelText("Candidate ID"), {
      target: { value: "not-a-uuid" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Apply filters" }));

    expect(
      await screen.findByText(
        "Enter a complete ID, such as 3f2c…-…, or leave the field empty.",
      ),
    ).toBeVisible();
    expect(screen.getByLabelText("Candidate ID")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(mocks.list.mock.calls.length).toBe(callsBefore);
  });

  it("pages forward and requests the next page", async () => {
    mocks.list.mockImplementation(async (params: { page: number }) =>
      params.page === 1
        ? page([recruitmentRow], 25, 1)
        : page([practiceRow], 25, 2),
    );
    renderScreen();
    await screen.findByRole("table", { name: "Interview sessions" });

    fireEvent.click(screen.getByRole("button", { name: "Next page" }));

    await waitFor(() =>
      expect(mocks.list).toHaveBeenLastCalledWith(
        expect.objectContaining({ page: 2 }),
      ),
    );
    expect(await screen.findByText("sam@example.com")).toBeVisible();
  });

  it("shows the empty state, and the filtered empty state after a filter", async () => {
    mocks.list.mockResolvedValue(page([], 0));
    renderScreen();

    expect(await screen.findByText("No interview sessions yet")).toBeVisible();

    fireEvent.change(screen.getByLabelText("Candidate ID"), {
      target: { value: candidateId },
    });
    fireEvent.click(screen.getByRole("button", { name: "Apply filters" }));

    expect(
      await screen.findByText("No matching interview sessions"),
    ).toBeVisible();
  });

  it("shows a loading state while the first request is pending", () => {
    mocks.list.mockReturnValue(new Promise(() => {}));
    renderScreen();

    expect(
      screen.getByRole("status", { name: "Loading interview sessions" }),
    ).toBeInTheDocument();
  });

  it("shows an error state that retries the request", async () => {
    mocks.list.mockRejectedValueOnce(apiError(500, "INTERNAL"));
    renderScreen();

    expect(
      await screen.findByText(
        "Unable to load interview sessions. Please try again.",
      ),
    ).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "Retry" }));

    expect(await screen.findByText("2 interview sessions")).toBeVisible();
  });

  it("hides interview data and offers an access check when admin access is refused", async () => {
    mocks.list.mockRejectedValue(apiError(403, "FORBIDDEN"));
    renderScreen();

    expect(
      await screen.findByText(/We couldn.t confirm your administrator access/),
    ).toBeVisible();
    expect(
      screen.queryByRole("table", { name: "Interview sessions" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Apply filters" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Retry access check" }),
    ).toBeVisible();
  });
});
