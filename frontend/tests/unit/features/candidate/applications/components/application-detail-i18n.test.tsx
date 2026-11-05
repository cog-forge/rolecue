import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { CandidateApplicationDetail } from "@/features/candidate/applications/components/application-detail";

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

const id = "44444444-4444-4444-8444-444444444444";
const application = (over: Record<string, unknown> = {}) => ({
  data: {
    success: true,
    data: {
      id,
      job_posting_id: "33333333-3333-4333-8333-333333333333",
      job_posting_title: "Go Backend Engineer",
      company_name: "Acme Robotics",
      job_posting_location: "Hanoi",
      status: "pending",
      reject_reason: null,
      interview_deadline: null,
      can_start_interview: false,
      created_at: "2026-10-20T08:00:00Z",
      ...over,
    },
  },
});

function renderDetail() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  return renderWithIntl(
    <QueryClientProvider client={client}>
      <CandidateApplicationDetail id={id} />
    </QueryClientProvider>,
    { locale: "vi" },
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
});

describe("CandidateApplicationDetail in Vietnamese", () => {
  it("translates status, facts and progress while the posting stays as stored", async () => {
    mocks.get.mockResolvedValue(application());
    renderDetail();
    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Go Backend Engineer",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText("Acme Robotics · Hanoi")).toBeInTheDocument();
    expect(screen.getByText("Đơn ứng tuyển")).toBeInTheDocument();
    expect(screen.getByText("Đang chờ xét duyệt CV")).toBeInTheDocument();
    expect(screen.getByText("Trạng thái")).toBeInTheDocument();
    expect(screen.getByText("20 thg 10, 2026")).toBeInTheDocument();
    expect(
      screen.getByText(/CV của bạn đang chờ được xét duyệt/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Quay lại đơn ứng tuyển của tôi" }),
    ).toBeInTheDocument();

    const progress = screen.getByRole("list", {
      name: "Tiến trình đơn ứng tuyển",
    });
    expect(progress).toHaveTextContent("Đã nộp: hoàn tất");
    expect(progress).toHaveTextContent("Duyệt CV: đang diễn ra");
    expect(progress).toHaveTextContent("Phỏng vấn: chưa tới");
    expect(progress).toHaveTextContent("Quyết định: chưa tới");
  });

  it("translates the interview result and the recruiter reason, which stays as written", async () => {
    mocks.get.mockResolvedValue(
      application({
        status: "rejected",
        reject_reason: "Needs more Kubernetes",
        interview: {
          id: "55555555-5555-4555-8555-555555555555",
          completed_at: "2026-10-22T08:00:00Z",
          status: "completed",
        },
      }),
    );
    renderDetail();
    expect(await screen.findByText("Không được chọn")).toBeInTheDocument();
    expect(screen.getByText("Kết quả phỏng vấn")).toBeInTheDocument();
    expect(screen.getByText("Đã hoàn thành")).toBeInTheDocument();
    expect(screen.queryByText(/72/)).not.toBeInTheDocument();
    expect(
      screen.getByText("Lý do: Needs more Kubernetes"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Đơn ứng tuyển này không được chọn cho vị trí này."),
    ).toBeInTheDocument();
  });

  it("offers Vietnamese start and resume actions", async () => {
    mocks.get.mockResolvedValue(
      application({
        status: "interview_eligible",
        interview_deadline: "2099-01-01T08:00:00Z",
        can_start_interview: true,
        interview: {
          id: "55555555-5555-4555-8555-555555555555",
          completed_at: null,
          status: "in_progress",
        },
      }),
    );
    renderDetail();
    expect(
      await screen.findByRole("link", { name: "Tiếp tục phỏng vấn" }),
    ).toHaveAttribute("href", `/interviews/new/preflight?applicationId=${id}`);
    expect(
      screen.getByText(/không tốn thêm lượt phỏng vấn mới/),
    ).toBeInTheDocument();
    expect(screen.getByText("Hạn phỏng vấn")).toBeInTheDocument();
    expect(screen.getByText("Thời gian còn lại")).toBeInTheDocument();
  });
});
