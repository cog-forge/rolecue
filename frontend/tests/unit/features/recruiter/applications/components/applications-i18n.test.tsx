import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { ApplicationDetail } from "@/features/recruiter/applications/components/application-detail";
import { ApplicationList } from "@/features/recruiter/applications/components/application-list";
import { ReviewPanel } from "@/features/recruiter/applications/components/review-panel";
import { ApplicationStatus } from "@/features/recruiter/applications/components/application-status";
import { applicationStatuses } from "@/features/recruiter/applications/schemas/application-schema";

// Radix Select calls scrollIntoView when it opens; jsdom does not implement it.
if (!HTMLElement.prototype.scrollIntoView)
  Object.defineProperty(HTMLElement.prototype, "scrollIntoView", {
    configurable: true,
    value: () => {},
  });

// Deadlines show in the viewer's time zone; pin one (before the components are imported).
vi.hoisted(() => {
  process.env.TZ = "Asia/Ho_Chi_Minh";
});

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  push: vi.fn(),
  search: new URLSearchParams(),
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
vi.mock("@/lib/api/client", () => ({
  apiClient: { get: mocks.get, post: mocks.post },
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mocks.push }),
  useSearchParams: () => mocks.search,
  usePathname: () => "/recruiter/applications",
}));

const postingId = "33333333-3333-4333-8333-333333333333";
const applicationId = "44444444-4444-4444-8444-000000000001";
const interviewId = "66666666-6666-4666-8666-000000000001";
const cvUrl = "https://files.example.com/cv/linh.pdf?signature=test";

const detail = (
  over: Record<string, unknown> = {},
  posting: Record<string, unknown> = {},
) => ({
  id: applicationId,
  job_posting_id: postingId,
  job_posting_title: "Backend Engineer",
  status: "pending",
  submitted_at: "2026-10-20T08:00:00Z",
  candidate: {
    id: "55555555-5555-4555-8555-000000000001",
    name: "Linh Tran",
    email: "linh@example.com",
    image: null,
  },
  interview_eligible: false,
  interview_deadline: null,
  reject_reason: null,
  updated_at: "2026-10-20T08:00:00Z",
  job_posting: {
    id: postingId,
    title: "Backend Engineer",
    status: "open",
    interview_slot: 5,
    cv_approvals: 1,
    ...posting,
  },
  cv: { url: cvUrl, expires_at: "2099-10-24T10:05:00Z" },
  ...over,
});
const row = (i: number, over: Record<string, unknown> = {}) => ({
  id: `44444444-4444-4444-8444-${String(i).padStart(12, "0")}`,
  job_posting_id: postingId,
  job_posting_title: "Backend Engineer",
  status: "pending",
  submitted_at: "2026-10-20T08:00:00Z",
  candidate: {
    id: `55555555-5555-4555-8555-${String(i).padStart(12, "0")}`,
    name: `Candidate ${i}`,
    email: `candidate${i}@example.com`,
    image: null,
  },
  interview_eligible: false,
  interview_deadline: null,
  reject_reason: null,
  updated_at: "2026-10-20T08:00:00Z",
  ...over,
});
const listPage = (items: ReturnType<typeof row>[], total = items.length) => ({
  data: {
    success: true,
    data: { items, pagination: { limit: 20, offset: 0, total } },
  },
});
const ok = (data: unknown) => ({ data: { success: true, data } });
function apiFailure(status: number, code: string) {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: { success: false, error: { code, message: "English from server" } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}
const interview = {
  id: interviewId,
  started_at: "2026-10-21T08:00:00Z",
  completed_at: "2026-10-21T08:30:00Z",
  status: "completed",
  score: 82,
  record_status: "ready",
};

function client() {
  const query = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  query.setQueryData(["auth", "me"], mocks.user);
  return query;
}
function mountDetail() {
  renderWithIntl(
    <QueryClientProvider client={client()}>
      <ApplicationDetail postingId={postingId} applicationId={applicationId} />
    </QueryClientProvider>,
    { locale: "vi" },
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
  mocks.post.mockReset();
  mocks.search = new URLSearchParams();
});

describe("ApplicationStatus", () => {
  it.each([
    ["pending", "Chờ xét duyệt CV"],
    ["interview_eligible", "Đủ điều kiện phỏng vấn"],
    ["interviewed", "Đã phỏng vấn"],
    ["approved", "Đã duyệt"],
    ["rejected", "Đã từ chối"],
  ] as const)("labels %s in Vietnamese", (status, label) => {
    renderWithIntl(<ApplicationStatus status={status} />, { locale: "vi" });
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  it("has a Vietnamese and an English label for every status", () => {
    for (const locale of ["en", "vi"] as const) {
      const view = renderWithIntl(
        <>
          {applicationStatuses.map((status) => (
            <ApplicationStatus key={status} status={status} />
          ))}
        </>,
        { locale },
      );
      expect(view.container.textContent).not.toContain("recruiter.");
      view.unmount();
    }
  });
});

describe("ApplicationList in Vietnamese", () => {
  function mountList() {
    mocks.get.mockImplementation((url: string) =>
      url === "/job-postings"
        ? Promise.resolve(listPage([], 0))
        : Promise.resolve(
            listPage([
              row(1),
              row(2, {
                status: "interview_eligible",
                interview_eligible: true,
                interview_deadline: "2026-10-21T08:30:00Z",
              }),
              row(3, {
                status: "interview_eligible",
                interview_eligible: false,
                interview_deadline: "2026-10-19T08:30:00Z",
              }),
              row(4, { status: "rejected", reject_reason: "Not a fit" }),
            ]),
          ),
    );
    renderWithIntl(
      <QueryClientProvider client={client()}>
        <ApplicationList postingId={postingId} />
      </QueryClientProvider>,
      { locale: "vi" },
    );
  }

  it("translates chrome, status, dates and aria-labels but not candidate data", async () => {
    mountList();
    expect(screen.getByRole("status")).toHaveTextContent(
      "Đang tải đơn ứng tuyển…",
    );
    const list = await screen.findByRole("list", {
      name: "Đơn ứng tuyển, 1–4 trên 4",
    });
    expect(within(list).getByText("Candidate 1")).toBeInTheDocument();
    expect(
      within(list).getByText("candidate1@example.com"),
    ).toBeInTheDocument();
    expect(within(list).getByText("Chờ xét duyệt CV")).toBeInTheDocument();
    expect(within(list).getByText("Đã từ chối")).toBeInTheDocument();
    expect(
      within(list).getByRole("link", { name: "Xét duyệt CV của Candidate 1" }),
    ).toBeInTheDocument();
    expect(
      within(list).getByRole("link", {
        name: "Mở đơn ứng tuyển của Candidate 4",
      }),
    ).toBeInTheDocument();
    expect(within(list).getAllByText(/^Nộp/)[0]).toHaveTextContent(
      "Nộp 20 thg 10, 2026",
    );
    expect(
      within(list).getByText(/Hoàn tất phỏng vấn trước/),
    ).toHaveTextContent("Hoàn tất phỏng vấn trước 15:30 GMT+7 21 thg 10, 2026");
    expect(within(list).getByText(/Đã quá hạn/)).toBeInTheDocument();
    expect(within(list).getByText("Lý do: Not a fit")).toBeInTheDocument();
    expect(screen.getByLabelText("Trạng thái")).toBeInTheDocument();
    expect(
      screen.getByRole("search", { name: "Tìm đơn ứng tuyển" }),
    ).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText("Tìm theo tên hoặc email"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("navigation", { name: "Phân trang đơn ứng tuyển" }),
    ).toHaveTextContent("Hiển thị 1–4 trên 4");
    expect(screen.getByRole("button", { name: "Trước" })).toBeDisabled();
  });

  it("explains an empty filtered list in Vietnamese", async () => {
    mocks.search = new URLSearchParams("status=approved");
    mocks.get.mockResolvedValue(listPage([], 0));
    renderWithIntl(
      <QueryClientProvider client={client()}>
        <ApplicationList postingId={postingId} />
      </QueryClientProvider>,
      { locale: "vi" },
    );
    expect(
      await screen.findByText("Bạn chưa duyệt ứng viên nào."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Hiện tất cả đơn ứng tuyển" }),
    ).toBeInTheDocument();
  });

  it("names the search in the empty state without translating the query", async () => {
    mocks.search = new URLSearchParams("q=Nguyen");
    mocks.get.mockResolvedValue(listPage([], 0));
    renderWithIntl(
      <QueryClientProvider client={client()}>
        <ApplicationList postingId={postingId} />
      </QueryClientProvider>,
      { locale: "vi" },
    );
    expect(
      await screen.findByText("Không có đơn ứng tuyển nào khớp với “Nguyen”."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Xóa tìm kiếm" }),
    ).toBeInTheDocument();
  });

  it("maps a load failure to Vietnamese copy", async () => {
    mocks.get.mockRejectedValue(apiFailure(500, "INTERNAL_ERROR"));
    renderWithIntl(
      <QueryClientProvider client={client()}>
        <ApplicationList postingId={postingId} />
      </QueryClientProvider>,
      { locale: "vi" },
    );
    expect(
      await screen.findByText("Không thể tải danh sách đơn ứng tuyển"),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Thử lại" })).toBeInTheDocument();
  });
});

describe("ApplicationDetail in Vietnamese", () => {
  it("translates the screening screen and keeps candidate data as stored", async () => {
    mocks.get.mockResolvedValue(ok(detail()));
    mountDetail();
    expect(screen.getByRole("status")).toHaveTextContent(
      "Đang tải đơn ứng tuyển…",
    );
    expect(
      await screen.findByRole("heading", { level: 1, name: "Linh Tran" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Ứng tuyển vị trí Backend Engineer"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Ứng viên" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Đơn ứng tuyển" }),
    ).toBeInTheDocument();
    expect(screen.getByText("linh@example.com")).toBeInTheDocument();
    expect(screen.getByText("Chờ xét duyệt CV")).toBeInTheDocument();
    expect(screen.getByText("20 thg 10, 2026")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Mở CV/ })).toHaveAttribute(
      "href",
      cvUrl,
    );
    expect(screen.getByTitle("CV của Linh Tran")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Duyệt CV" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Từ chối CV" })).toBeEnabled();
    expect(
      screen.getByRole("link", { name: "Về danh sách đơn ứng tuyển" }),
    ).toBeInTheDocument();
  });

  it("explains why approval is blocked", async () => {
    mocks.get.mockResolvedValue(
      ok(detail({}, { interview_slot: null, cv_approvals: 0 })),
    );
    mountDetail();
    await screen.findByRole("heading", { level: 1, name: "Linh Tran" });
    expect(screen.getByText("Chưa thể duyệt CV")).toBeInTheDocument();
    expect(
      screen.getByText(
        /chưa được nạp lượt phỏng vấn nên chưa thể duyệt CV nào/,
      ),
    ).toBeInTheDocument();
  });

  it("confirms the last slot in Vietnamese", async () => {
    mocks.get.mockResolvedValue(
      ok(detail({}, { interview_slot: 2, cv_approvals: 1 })),
    );
    mountDetail();
    await screen.findByRole("heading", { level: 1, name: "Linh Tran" });
    fireEvent.click(screen.getByRole("button", { name: "Duyệt CV" }));
    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByText("Duyệt cho lượt phỏng vấn cuối cùng?"),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByText(/^Linh Tran sẽ nhận lượt phỏng vấn cuối cùng/),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByRole("button", { name: "Hủy" }),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByRole("button", { name: "Xác nhận duyệt" }),
    ).toBeInTheDocument();
  });

  it("validates the reject reason in Vietnamese and sends the rejection", async () => {
    mocks.get.mockResolvedValue(ok(detail()));
    mountDetail();
    await screen.findByRole("heading", { level: 1, name: "Linh Tran" });
    fireEvent.click(screen.getByRole("button", { name: "Từ chối CV" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Từ chối CV này?")).toBeInTheDocument();
    const reason = within(dialog).getByLabelText(/Lý do gửi ứng viên/);
    fireEvent.change(reason, { target: { value: "x".repeat(501) } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Từ chối CV" }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Lý do tối đa 500 ký tự.",
    );
    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("shows a translated error chosen by code, never the server text", async () => {
    mocks.get.mockResolvedValue(ok(detail()));
    mocks.post.mockRejectedValue(apiFailure(409, "APPLICATION_NOT_PENDING"));
    mountDetail();
    await screen.findByRole("heading", { level: 1, name: "Linh Tran" });
    fireEvent.click(screen.getByRole("button", { name: "Duyệt CV" }));
    expect(
      await screen.findByText(
        "Đơn ứng tuyển này đã được quyết định. Hãy tải lại để xem trạng thái hiện tại.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("English from server")).not.toBeInTheDocument();
  });

  it("shows the interview result, evidence and a locked final decision", async () => {
    mocks.get.mockImplementation((url: string) => {
      if (url.endsWith("/transcript"))
        return Promise.resolve(
          ok({
            turns: [
              {
                position: 1,
                question_number: 1,
                kind: "core",
                question: "Tell me about Go",
                answer: null,
              },
            ],
          }),
        );
      if (url.endsWith("/report"))
        return Promise.resolve(
          ok({
            interview_id: interviewId,
            type: "recruitment",
            status: "completed",
            overall_score: 82,
            started_at: "2026-10-21T08:00:00Z",
            completed_at: "2026-10-21T08:30:00Z",
            summary: "Strong on concurrency.",
            scoring: {
              method: "rubric",
              outcome: "complete",
              withheld_reason: null,
              assigned_questions: 1,
              graded_questions: 1,
              weight_total: 1,
              decimals: 2,
            },
            breakdown: [
              {
                metric_id: "77777777-7777-4777-8777-000000000001",
                name: "Communication",
                score: 82,
                weight: 1,
                question_count: 1,
              },
            ],
            graded_questions: [],
            questions: null,
          }),
        );
      return Promise.resolve(
        ok(
          detail({
            status: "interviewed",
            interview,
            final_review: {
              opens_at: "2099-10-25T08:00:00Z",
              can_decide: false,
            },
          }),
        ),
      );
    });
    mountDetail();
    await screen.findByRole("heading", { level: 1, name: "Linh Tran" });
    expect(
      screen.getByRole("heading", { name: "Phỏng vấn" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Đã đánh giá")).toBeInTheDocument();
    expect(screen.getByText("82,00/100")).toBeInTheDocument();
    expect(screen.getByText("Đã có bản ghi hình.")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Quyết định cuối cùng" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Chưa thể đưa ra quyết định cuối cùng"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/^Quyết định sẽ mở vào .*25 thg 10, 2099/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Duyệt đơn ứng tuyển" }),
    ).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Xem đánh giá" }));
    expect(
      await screen.findByText("Strong on concurrency."),
    ).toBeInTheDocument();
    expect(screen.getByText("Communication")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Xem bản ghi lời thoại" }),
    );
    expect(await screen.findByText("Câu hỏi 1")).toBeInTheDocument();
    expect(screen.getByText("Tell me about Go")).toBeInTheDocument();
    expect(
      screen.getByText("Chưa có câu trả lời nào được ghi lại."),
    ).toBeInTheDocument();
  });

  it("shows Vietnamese denied and not-found states", async () => {
    mocks.get.mockRejectedValue(apiFailure(404, "APPLICATION_NOT_FOUND"));
    mountDetail();
    expect(
      await screen.findByText("Không tìm thấy đơn ứng tuyển này"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Về đơn ứng tuyển đã nhận" }),
    ).toBeInTheDocument();
  });
});

describe("ReviewPanel in Vietnamese", () => {
  it("translates the panel navigation, facts and slot counter", async () => {
    mocks.get.mockResolvedValue(
      ok(
        detail({
          status: "interview_eligible",
          interview_eligible: true,
          interview_deadline: "2026-10-21T08:30:00Z",
        }),
      ),
    );
    renderWithIntl(
      <QueryClientProvider client={client()}>
        <ReviewPanel
          postingId={postingId}
          applicationId={applicationId}
          position={{ index: 1, count: 4 }}
          onPrevious={() => {}}
          onNext={() => {}}
          onDecided={() => {}}
          caughtUp
        />
      </QueryClientProvider>,
      { locale: "vi" },
    );
    expect(
      await screen.findByRole("heading", { name: "Linh Tran" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Đơn ứng tuyển trước" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: "Đơn ứng tuyển sau" }),
    ).toBeEnabled();
    expect(screen.getByText("2 trên 4")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Mở đơn ứng tuyển ở trang riêng" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Lượt phỏng vấn")).toBeInTheDocument();
    expect(screen.getByText("Đã duyệt 1 trên 5")).toBeInTheDocument();
    expect(screen.getByText("Hoàn tất phỏng vấn trước")).toBeInTheDocument();
    expect(screen.getByText("Bạn đã xem hết")).toBeInTheDocument();
    expect(screen.getByText(/^Đã được mời phỏng vấn\./)).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByRole("heading", { name: "CV" })).toBeInTheDocument(),
    );
  });
});
