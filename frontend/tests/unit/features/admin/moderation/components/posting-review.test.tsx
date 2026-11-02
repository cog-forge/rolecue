import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import type { AppLocale } from "@/lib/i18n/config";
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
vi.mock("@/lib/api/client", () => ({
  apiClient: { get: mocks.get, post: mocks.post },
}));
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
  document_url: null,
  document_expires_at: null,
  raw_text: null,
  review_submitted: true,
  core_question_count: 50,
  company_name: "Cog Forge",
  company_website: "https://cogforge.example",
  seniority_level: "senior",
  refinement_note: "Focus on production debugging.",
  requirements: [
    { name: "Go", target: "programming_language", type: "required" },
    { name: "PostgreSQL", target: "database", type: "preferred" },
  ],
  weights_configured: false,
  evaluation: {
    configured: false,
    using_defaults: true,
    total: 100,
    weights: [
      {
        metric_id: "66666666-6666-4666-8666-666666666661",
        name: "Technical correctness",
        percentage: 50,
      },
      {
        metric_id: "66666666-6666-4666-8666-666666666662",
        name: "Communication",
        percentage: 50,
      },
    ],
  },
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
function mount(locale: AppLocale = "en") {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity },
      mutations: { retry: false },
    },
  });
  client.setQueryData(["auth", "me"], mocks.user);
  renderWithIntl(
    <QueryClientProvider client={client}>
      <PostingReview id={id} backQuery="status=all" />
    </QueryClientProvider>,
    { locale },
  );
  return client;
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
  mocks.post.mockReset();
});

describe("PostingReview", () => {
  it("shows confirmed requirements and effective default criteria for read-only moderation", async () => {
    mocks.get.mockResolvedValue(envelope(posting()));
    mount();

    expect(
      await screen.findByRole("heading", { name: "Confirmed requirements" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Go", { exact: true })).toBeInTheDocument();
    expect(screen.getByText("PostgreSQL", { exact: true })).toBeInTheDocument();
    expect(screen.getByText("Cog Forge")).toBeInTheDocument();
    expect(screen.getByText("senior", { exact: true })).toBeInTheDocument();
    expect(screen.getByText("30 minutes")).toBeInTheDocument();
    expect(screen.getByText("Created", { exact: true })).toBeInTheDocument();
    expect(
      screen.getByText("Focus on production debugging."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Evaluation criteria" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/default.*equal/i)).toBeInTheDocument();
    expect(screen.getByText("Technical correctness")).toBeInTheDocument();
    expect(screen.getByText("Communication")).toBeInTheDocument();
    expect(screen.getAllByText("50%")).toHaveLength(2);
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    expect(screen.queryByRole("spinbutton")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /save|edit criteria/i }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Approve" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Reject" })).toBeEnabled();
  });

  it("shows the recruiter's configured percentages rather than substituting defaults", async () => {
    mocks.get.mockResolvedValue(
      envelope(
        posting({
          weights_configured: true,
          evaluation: {
            configured: true,
            using_defaults: false,
            total: 100,
            weights: [
              {
                metric_id: "66666666-6666-4666-8666-666666666661",
                name: "Technical correctness",
                percentage: 70,
              },
              {
                metric_id: "66666666-6666-4666-8666-666666666662",
                name: "Communication",
                percentage: 30,
              },
            ],
          },
        }),
      ),
    );
    mount();

    expect(
      await screen.findByRole("heading", { name: "Evaluation criteria" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/custom evaluation criteria/i)).toBeInTheDocument();
    expect(screen.getByText("70%")).toBeInTheDocument();
    expect(screen.getByText("30%")).toBeInTheDocument();
    expect(screen.queryByText("50%")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /save|edit criteria/i }),
    ).not.toBeInTheDocument();
  });

  it("shows the posting, the JD file and the question count", async () => {
    mocks.get.mockResolvedValue(envelope(posting()));
    mount();
    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Go Backend Engineer",
      }),
    ).toBeInTheDocument();
    expect(screen.getByTitle("Job description PDF")).toBeInTheDocument();
    expect(
      within(screen.getByRole("region", { name: "Decision" })).getByText("50"),
    ).toBeInTheDocument();
    expect(screen.getByText("1,500 – 2,500")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /back to moderation queue/i }),
    ).toHaveAttribute("href", "/admin/job-postings?status=all");
  });

  it("disables Approve with an explanation when the bank is empty", async () => {
    mocks.get.mockResolvedValue(envelope(posting({ core_question_count: 0 })));
    mount();
    const approve = await screen.findByRole("button", { name: "Approve" });
    expect(approve).toBeDisabled();
    expect(approve).toHaveAccessibleDescription(
      /recruiter must add core questions/i,
    );
    // A submitted posting can still be sent back, whatever its bank holds.
    expect(screen.getByRole("button", { name: "Reject" })).toBeEnabled();
  });

  it("cannot decide a posting the recruiter has not submitted", async () => {
    mocks.get.mockResolvedValue(envelope(posting({ review_submitted: false })));
    mount();
    const approve = await screen.findByRole("button", { name: "Approve" });
    expect(approve).toBeDisabled();
    expect(approve).toHaveAccessibleDescription(
      /still preparing this posting/i,
    );
    expect(screen.getByRole("button", { name: "Reject" })).toBeDisabled();
  });

  it("approves after confirmation", async () => {
    mocks.get.mockResolvedValue(envelope(posting()));
    mocks.post.mockResolvedValue(envelope(posting({ status: "open" })));
    mount();
    fireEvent.click(await screen.findByRole("button", { name: "Approve" }));
    const dialog = await screen.findByRole("dialog", {
      name: /approve this posting/i,
    });
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Approve posting" }),
    );
    await waitFor(() =>
      expect(mocks.post).toHaveBeenCalledWith(
        `/admin/job-postings/${id}/approve`,
      ),
    );
    expect(await screen.findByText("Approved")).toBeInTheDocument();
    expect(mocks.toast).toHaveBeenCalledWith(
      "Posting approved",
      expect.objectContaining({
        description:
          "Candidates can see it once the recruiter funds interview slots.",
      }),
    );
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
  });

  it("requires a reason to reject and sends exactly what Go accepts", async () => {
    mocks.get.mockResolvedValue(envelope(posting()));
    mocks.post.mockResolvedValue(
      envelope(
        posting({ status: "rejected", reject_reason: "Salary is missing" }),
      ),
    );
    mount();
    fireEvent.click(await screen.findByRole("button", { name: "Reject" }));
    const dialog = await screen.findByRole("dialog", {
      name: /reject this posting/i,
    });
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Reject posting" }),
    );
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      /tell the recruiter why/i,
    );
    expect(mocks.post).not.toHaveBeenCalled();
    const reason = within(dialog).getByLabelText(/reason for the recruiter/i);
    expect(reason).toHaveAttribute("aria-invalid", "true");
    fireEvent.change(reason, { target: { value: "  Salary is missing  " } });
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Reject posting" }),
    );
    await waitFor(() =>
      expect(mocks.post).toHaveBeenCalledWith(
        `/admin/job-postings/${id}/reject`,
        {
          reject_reason: "Salary is missing",
        },
      ),
    );
    expect(await screen.findByText(/“Salary is missing”/)).toBeInTheDocument();
  });

  it("keeps the reason and shows translated text, never the server message, when rejecting fails", async () => {
    mocks.get.mockResolvedValue(envelope(posting()));
    mocks.post.mockRejectedValue(
      failure(500, "INTERNAL_ERROR", "Database is down"),
    );
    mount();
    fireEvent.click(await screen.findByRole("button", { name: "Reject" }));
    const dialog = await screen.findByRole("dialog");
    const reason = within(dialog).getByLabelText(/reason for the recruiter/i);
    fireEvent.change(reason, { target: { value: "Duplicate posting" } });
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Reject posting" }),
    );
    expect(
      await within(dialog).findByText(
        "We couldn't save your decision. Please try again.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Database is down")).not.toBeInTheDocument();
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
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Approve posting" }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "This posting isn't waiting for review.",
    );
    await waitFor(() => expect(mocks.get).toHaveBeenCalledTimes(2));
    expect(
      await screen.findByText(/already been decided/i),
    ).toBeInTheDocument();
  });

  it("shows not found without a retry for a missing posting", async () => {
    mocks.get.mockRejectedValue(
      failure(404, "JOB_POSTING_NOT_FOUND", "job posting not found"),
    );
    mount();
    expect(await screen.findByRole("alert")).toHaveTextContent(
      /doesn't exist/i,
    );
    expect(
      screen.queryByRole("button", { name: "Try again" }),
    ).not.toBeInTheDocument();
  });

  it("renders the review screen in Vietnamese, leaving posting content as stored", async () => {
    mocks.get.mockResolvedValue(envelope(posting()));
    mount("vi");
    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Go Backend Engineer",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText("Chờ duyệt")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Yêu cầu đã xác nhận" }),
    ).toBeInTheDocument();
    // Requirement names and targets are recruiter content; only the required/preferred label is translated.
    expect(screen.getByText("Go", { exact: true })).toBeInTheDocument();
    expect(screen.getByText("programming language")).toBeInTheDocument();
    expect(screen.getByText("Bắt buộc", { exact: false })).toBeInTheDocument();
    expect(
      screen.getByText("Focus on production debugging."),
    ).toBeInTheDocument();
    expect(screen.getByText("Cog Forge")).toBeInTheDocument();
    expect(screen.getByText("30 phút")).toBeInTheDocument();
    expect(screen.getByText("1.500 – 2.500")).toBeInTheDocument();
    expect(screen.getByText(/12 thg 10, 2026.*UTC/)).toBeInTheDocument();
    const decision = screen.getByRole("region", { name: "Quyết định" });
    expect(within(decision).getByText("câu hỏi cốt lõi")).toBeInTheDocument();
    expect(
      within(decision).getByRole("button", { name: "Duyệt" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("link", { name: /quay lại hàng chờ/i }),
    ).toHaveAttribute("href", "/admin/job-postings?status=all");
    expect(screen.getByText("Chưa đặt")).toBeInTheDocument();
    expect(screen.getAllByText("50%")).toHaveLength(2);
  });

  it("confirms a rejection in Vietnamese and validates the reason", async () => {
    mocks.get.mockResolvedValue(envelope(posting()));
    mocks.post.mockResolvedValue(
      envelope(
        posting({ status: "rejected", reject_reason: "Thiếu mức lương" }),
      ),
    );
    mount("vi");
    fireEvent.click(await screen.findByRole("button", { name: "Từ chối" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Từ chối tin tuyển dụng này?",
    });
    expect(dialog).toHaveTextContent(
      "Go Backend Engineer sẽ không được đăng. Nhà tuyển dụng sẽ xem được lý do của bạn.",
    );
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Từ chối tin" }),
    );
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Hãy cho nhà tuyển dụng biết vì sao tin tuyển dụng này bị từ chối.",
    );
    fireEvent.change(
      within(dialog).getByLabelText(/lý do gửi nhà tuyển dụng/i),
      {
        target: { value: "Thiếu mức lương" },
      },
    );
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Từ chối tin" }),
    );
    await waitFor(() =>
      expect(mocks.post).toHaveBeenCalledWith(
        `/admin/job-postings/${id}/reject`,
        { reject_reason: "Thiếu mức lương" },
      ),
    );
    expect(mocks.toast).toHaveBeenCalledWith(
      "Đã từ chối tin tuyển dụng",
      expect.objectContaining({
        description: "Nhà tuyển dụng sẽ xem được lý do của bạn.",
      }),
    );
    expect(await screen.findByText("Đã từ chối")).toBeInTheDocument();
    expect(await screen.findByText(/“Thiếu mức lương”/)).toBeInTheDocument();
  });

  it("maps a 409 by code to Vietnamese and never shows the backend message", async () => {
    mocks.get
      .mockResolvedValueOnce(envelope(posting()))
      .mockResolvedValue(envelope(posting({ status: "open" })));
    mocks.post.mockRejectedValue(
      failure(409, "JOB_POSTING_NOT_PENDING", "only a pending job posting"),
    );
    mount("vi");
    fireEvent.click(await screen.findByRole("button", { name: "Duyệt" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: "Duyệt tin" }));
    const alert = await screen.findByRole("alert");
    expect(alert).not.toHaveTextContent(/only a pending/i);
    expect(alert.textContent).toMatch(/\S/);
    expect(await screen.findByText(/đã được quyết định/i)).toBeInTheDocument();
  });

  it("translates the not-found state in Vietnamese", async () => {
    mocks.get.mockRejectedValue(
      failure(404, "JOB_POSTING_NOT_FOUND", "job posting not found"),
    );
    mount("vi");
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Tin tuyển dụng này không tồn tại",
    );
  });
});
