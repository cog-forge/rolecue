import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { PostingReview } from "@/features/admin/moderation/components/posting-review";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  toast: vi.fn(),
  user: {
    id: "11111111-1111-4111-8111-111111111111",
    full_name: "Admin",
    email: "admin@example.com",
    role: "admin" as const,
    email_verified: true,
    is_locked: false,
    image: null,
  },
}));
vi.mock("@/lib/api/client", () => ({ apiClient: { get: mocks.get, post: mocks.post } }));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));
vi.mock("goey-toast", () => ({ gooeyToast: { success: mocks.toast } }));

const id = "33333333-3333-4333-8333-333333333333";
const posting = (overrides: Record<string, unknown> = {}) => ({
  id,
  title: "Go Backend Engineer",
  status: "pending",
  reject_reason: null,
  recruiter: {
    id: "22222222-2222-4222-8222-222222222222",
    name: "Rec Ruiter",
    email: "rec@example.com",
  },
  location: "Ho Chi Minh City",
  employment_type: "Full-time",
  salary_min: 1500,
  salary_max: 2500,
  interview_slot: 10,
  interview_duration: 30,
  required_avatar: { id: "55555555-5555-4555-8555-555555555555", name: "Linh" },
  required_voice: null,
  created_at: "2026-10-12T08:00:00Z",
  content_path: "https://files.example.com/jd/go.pdf",
  core_question_count: 50,
  ...overrides,
});
const envelope = (data: unknown) => ({ data: { success: true, data } });
function failure(status: number, code: string, message: string) {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: { success: false, error: { code, message } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}
function mount() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity }, mutations: { retry: false } },
  });
  client.setQueryData(["auth", "me"], mocks.user);
  render(
    <QueryClientProvider client={client}>
      <PostingReview id={id} backQuery="status=all" />
    </QueryClientProvider>,
  );
  return client;
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
  mocks.post.mockReset();
});

describe("PostingReview", () => {
  it("shows the posting, the JD file and the question count", async () => {
    mocks.get.mockResolvedValue(envelope(posting()));
    mount();
    expect(
      await screen.findByRole("heading", { level: 1, name: "Go Backend Engineer" }),
    ).toBeInTheDocument();
    expect(screen.getByTitle("Job description PDF")).toBeInTheDocument();
    expect(screen.getByText(/core questions in the question bank/i)).toHaveTextContent("50");
    expect(screen.getByText("1,500 – 2,500")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /back to moderation queue/i })).toHaveAttribute(
      "href",
      "/admin/job-postings?status=all",
    );
  });

  it("disables Approve with an explanation when the bank is empty", async () => {
    mocks.get.mockResolvedValue(envelope(posting({ core_question_count: 0 })));
    mount();
    const approve = await screen.findByRole("button", { name: "Approve" });
    expect(approve).toBeDisabled();
    expect(approve).toHaveAccessibleDescription(
      /recruiter must add core questions before this posting can be approved/i,
    );
    expect(screen.getByRole("button", { name: "Reject" })).toBeEnabled();
  });

  it("approves after confirmation", async () => {
    mocks.get.mockResolvedValue(envelope(posting()));
    mocks.post.mockResolvedValue(envelope(posting({ status: "open" })));
    mount();
    fireEvent.click(await screen.findByRole("button", { name: "Approve" }));
    const dialog = await screen.findByRole("dialog", { name: /approve this posting/i });
    fireEvent.click(within(dialog).getByRole("button", { name: "Approve posting" }));
    await waitFor(() =>
      expect(mocks.post).toHaveBeenCalledWith(`/admin/job-postings/${id}/approve`),
    );
    expect(await screen.findByText("Approved")).toBeInTheDocument();
    expect(mocks.toast).toHaveBeenCalledWith("Posting approved", expect.any(Object));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("requires a reason to reject and sends exactly what Go accepts", async () => {
    mocks.get.mockResolvedValue(envelope(posting()));
    mocks.post.mockResolvedValue(
      envelope(posting({ status: "rejected", reject_reason: "Salary is missing" })),
    );
    mount();
    fireEvent.click(await screen.findByRole("button", { name: "Reject" }));
    const dialog = await screen.findByRole("dialog", { name: /reject this posting/i });
    fireEvent.click(within(dialog).getByRole("button", { name: "Reject posting" }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      /tell the recruiter why/i,
    );
    expect(mocks.post).not.toHaveBeenCalled();
    const reason = within(dialog).getByLabelText(/reason for the recruiter/i);
    expect(reason).toHaveAttribute("aria-invalid", "true");
    fireEvent.change(reason, { target: { value: "  Salary is missing  " } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Reject posting" }));
    await waitFor(() =>
      expect(mocks.post).toHaveBeenCalledWith(`/admin/job-postings/${id}/reject`, {
        reject_reason: "Salary is missing",
      }),
    );
    expect(await screen.findByText(/“Salary is missing”/)).toBeInTheDocument();
  });

  it("keeps the reason and shows the server message when rejecting fails", async () => {
    mocks.get.mockResolvedValue(envelope(posting()));
    mocks.post.mockRejectedValue(failure(500, "INTERNAL_ERROR", "Database is down"));
    mount();
    fireEvent.click(await screen.findByRole("button", { name: "Reject" }));
    const dialog = await screen.findByRole("dialog");
    const reason = within(dialog).getByLabelText(/reason for the recruiter/i);
    fireEvent.change(reason, { target: { value: "Duplicate posting" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Reject posting" }));
    expect(await within(dialog).findByText("Database is down")).toBeInTheDocument();
    expect(reason).toHaveValue("Duplicate posting");
  });

  it("explains a 409 and reloads the posting", async () => {
    mocks.get
      .mockResolvedValueOnce(envelope(posting()))
      .mockResolvedValue(envelope(posting({ status: "open" })));
    mocks.post.mockRejectedValue(
      failure(
        409,
        "JOB_POSTING_NOT_PENDING",
        "only a pending job posting can be approved or rejected",
      ),
    );
    mount();
    fireEvent.click(await screen.findByRole("button", { name: "Approve" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Approve posting" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      /only a pending job posting can be approved or rejected/i,
    );
    await waitFor(() => expect(mocks.get).toHaveBeenCalledTimes(2));
    expect(await screen.findByText(/already been decided/i)).toBeInTheDocument();
  });

  it("shows not found without a retry for a missing posting", async () => {
    mocks.get.mockRejectedValue(failure(404, "JOB_POSTING_NOT_FOUND", "job posting not found"));
    mount();
    expect(await screen.findByRole("alert")).toHaveTextContent(/doesn't exist/i);
    expect(screen.queryByRole("button", { name: "Try again" })).not.toBeInTheDocument();
  });
});
