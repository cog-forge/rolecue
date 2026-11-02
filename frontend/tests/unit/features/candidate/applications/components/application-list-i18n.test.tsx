import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { CandidateApplicationList } from "@/features/candidate/applications/components/application-list";

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
vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(),
}));

const item = (i: number, over: Record<string, unknown> = {}) => ({
  id: `44444444-4444-4444-8444-${String(i).padStart(12, "0")}`,
  job_posting_id: "33333333-3333-4333-8333-333333333333",
  job_posting_title: `Backend Engineer ${i}`,
  company_name: "Acme Robotics",
  job_posting_location: "Hanoi",
  status: "pending",
  reject_reason: null,
  interview_deadline: null,
  can_start_interview: false,
  created_at: "2026-10-20T08:00:00Z",
  ...over,
});
const envelope = (items: unknown[]) => ({
  data: {
    success: true,
    data: { items, total: items.length, page: 1, page_size: 20 },
  },
});

function renderList() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  return renderWithIntl(
    <QueryClientProvider client={client}>
      <CandidateApplicationList />
    </QueryClientProvider>,
    { locale: "vi" },
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
});

describe("CandidateApplicationList in Vietnamese", () => {
  it("translates status, progress, dates and actions but not posting data", async () => {
    mocks.get.mockResolvedValue(
      envelope([
        item(1),
        item(2, {
          status: "rejected",
          reject_reason: "Missing Go experience",
        }),
        item(3, {
          status: "interview_eligible",
          can_start_interview: true,
          interview_deadline: "2099-01-01T08:00:00Z",
        }),
      ]),
    );
    renderList();
    expect(screen.getByRole("status")).toHaveTextContent(
      "Đang tải đơn ứng tuyển của bạn…",
    );

    const list = await screen.findByRole("list", {
      name: "Đơn ứng tuyển của bạn",
    });
    const rows = within(list)
      .getAllByRole("listitem")
      .filter((row) => row.parentElement === list);
    expect(rows).toHaveLength(3);
    expect(rows[0]).toHaveTextContent("Đang chờ xét duyệt CV");
    expect(rows[1]).toHaveTextContent("Không được chọn");
    expect(rows[2]).toHaveTextContent("Đủ điều kiện phỏng vấn");
    // Submitted date in the Vietnamese calendar format.
    expect(rows[0]).toHaveTextContent("Đã nộp 20 thg 10, 2026");
    expect(rows[0]).toHaveTextContent("Duyệt CV");
    // Posting title, company, location and the recruiter's own note stay as stored.
    expect(
      screen.getByRole("link", { name: "Backend Engineer 1" }),
    ).toBeInTheDocument();
    expect(rows[0]).toHaveTextContent("Acme Robotics · Hanoi");
    expect(rows[1]).toHaveTextContent(
      "Ghi chú của nhà tuyển dụng: Missing Go experience",
    );
    expect(
      screen.getByRole("link", { name: /Bắt đầu phỏng vấn/ }),
    ).toHaveAttribute(
      "href",
      "/interviews/new/preflight?applicationId=44444444-4444-4444-8444-000000000003",
    );
    expect(
      screen.getByRole("link", {
        name: "Xem đơn ứng tuyển cho Backend Engineer 1",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText(/^Còn /)).toBeInTheDocument();
    expect(screen.getByText("Hiển thị 1–3 trên 3")).toBeInTheDocument();
    expect(
      screen.getAllByRole("list", { name: "Tiến trình đơn ứng tuyển" }),
    ).toHaveLength(3);
    expect(screen.queryByText("Awaiting CV review")).not.toBeInTheDocument();
  });

  it("translates the empty state", async () => {
    mocks.get.mockResolvedValue(envelope([]));
    renderList();
    expect(
      await screen.findByText("Bạn chưa ứng tuyển tin tuyển dụng nào."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Xem bảng tin tuyển dụng" }),
    ).toBeInTheDocument();
  });
});
