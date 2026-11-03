import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { ReportOverview } from "@/features/report/components/report-overview";

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
const METRIC = "55555555-5555-4555-8555-555555555551";

const report = (over: Record<string, unknown> = {}) => ({
  interview_id: INTERVIEW,
  type: "practice",
  status: "completed",
  overall_score: 72,
  started_at: "2026-10-20T08:00:00Z",
  completed_at: "2026-10-20T08:30:00Z",
  summary: "Clear structure, thin examples.",
  breakdown: [{ metric_id: METRIC, name: "Communication", score: 80 }],
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
function failure(status: number) {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: {
      success: false,
      error: { code: "INTERNAL_ERROR", message: "server says" },
    },
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
    { locale: "vi" },
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
});

describe("ReportOverview in Vietnamese", () => {
  it("says in Vietnamese that the Candidate left early", async () => {
    mocks.get.mockResolvedValue(
      envelope(report({ end_reason: "left", question_count: 5 })),
    );
    renderReport();
    expect(
      await screen.findByText("Bạn đã rời buổi phỏng vấn sớm"),
    ).toBeInTheDocument();
    expect(screen.getByTestId("report-end-notice")).toHaveTextContent(
      /\/5 câu; .* câu chưa trả lời\./,
    );
  });

  it("translates the chrome and keeps AI feedback, summary and questions as stored", async () => {
    mocks.get.mockResolvedValue(envelope(report()));
    renderReport();
    expect(screen.getByRole("status")).toHaveTextContent(
      "Đang tải báo cáo của bạn…",
    );

    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Phỏng vấn luyện tập",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText("Báo cáo phỏng vấn")).toBeInTheDocument();
    expect(screen.getByText("Hoàn tất")).toBeInTheDocument();
    expect(screen.getByText(/Bắt đầu lúc/)).toBeInTheDocument();
    expect(screen.getByText(/Hoàn tất lúc/)).toBeInTheDocument();
    const started = screen
      .getByText(/Bắt đầu lúc/)
      .querySelector("time") as HTMLElement;
    expect(started).toHaveAttribute("datetime", "2026-10-20T08:00:00Z");

    expect(
      screen.getByRole("heading", { name: "Điểm tổng thể" }),
    ).toBeInTheDocument();
    expect(screen.getByText("72 / 100")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Tóm tắt" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Điểm theo tiêu chí" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Câu hỏi" }),
    ).toBeInTheDocument();

    // AI and candidate text stay exactly as stored (English), only the labels around it change.
    expect(
      screen.getByText("Clear structure, thin examples."),
    ).toBeInTheDocument();
    const bar = screen.getByRole("progressbar", { name: "Communication" });
    expect(bar).toHaveAttribute("aria-valuetext", "80 trên 100");
    expect(
      within(bar.parentElement as HTMLElement).getByText("Communication"),
    ).toBeInTheDocument();
    expect(screen.getByText("Tell me about a project.")).toBeInTheDocument();
    expect(screen.getByText("I built an API.")).toBeInTheDocument();
    expect(screen.getByText("Name the impact.")).toBeInTheDocument();
    expect(screen.getAllByText("Câu trả lời:", { exact: false })).toHaveLength(
      2,
    );
    expect(screen.getByText("Nhận xét:", { exact: false })).toBeInTheDocument();
    expect(screen.getByText("Chưa ghi nhận câu trả lời.")).toBeInTheDocument();
    expect(
      screen.getAllByRole("link", { name: "Quay lại lịch sử phỏng vấn" }),
    ).toHaveLength(2);
  });

  it("translates the not-finished notices and their action", async () => {
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
        name: "Buổi phỏng vấn của bạn đang được đánh giá",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText("Đang đánh giá")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Kiểm tra lại" }),
    ).toBeInTheDocument();
  });

  it("translates the recruitment note and the failure states", async () => {
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
    const first = renderReport();
    expect(
      await screen.findByRole("heading", { name: "Phỏng vấn tuyển dụng" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Phản hồi chi tiết của phỏng vấn tuyển dụng không được hiển thị tại đây.",
      ),
    ).toBeInTheDocument();
    first.unmount();

    mocks.get.mockRejectedValue(failure(404));
    const second = renderReport();
    expect(
      await screen.findByRole("heading", {
        name: "Không tìm thấy báo cáo này",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Mở lịch sử phỏng vấn" }),
    ).toBeInTheDocument();
    second.unmount();

    mocks.get.mockRejectedValue(failure(500));
    renderReport();
    expect(
      await screen.findByRole("heading", { name: "Không thể tải báo cáo này" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("server says")).not.toBeInTheDocument();
  });

  it("explains an invalid link", () => {
    renderReport("nope");
    expect(
      screen.getByRole("heading", { name: "Liên kết này không hợp lệ" }),
    ).toBeInTheDocument();
  });
});
