import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, within } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { JobPostingDetail } from "@/features/job-board/components/job-posting-detail";

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

const id = "33333333-3333-4333-8333-333333333333";
const skills = Array.from({ length: 12 }, (_, i) => ({
  name: `Skill ${i + 1}`,
  target: "tool",
  type: "required",
}));
const detail = (extra: Record<string, unknown> = {}) => ({
  data: {
    success: true,
    data: {
      id,
      title: "Go Backend Engineer",
      company_name: "Acme",
      company_website: "https://acme.example.com",
      location: "Hanoi",
      employment_type: "Full-time",
      salary_min: 10_000_000,
      salary_max: 20_000_000,
      seniority_level: "mid",
      interview_duration: 30,
      created_at: "2026-10-12T08:00:00Z",
      raw_text: "We build reliable APIs in Go.",
      requirements: [
        { name: "Go", target: "programming_language", type: "required" },
        { name: "Kubernetes", target: "cloud_platform", type: "preferred" },
      ],
      already_applied: false,
      ...extra,
    },
  },
});
function failure(status: number) {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: { success: false, error: { code: "X", message: "server says" } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}
function mount(postingId = id) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  client.setQueryData(["auth", "me"], mocks.user);
  return renderWithIntl(
    <QueryClientProvider client={client}>
      <JobPostingDetail id={postingId} />
    </QueryClientProvider>,
    { locale: "vi" },
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
  Object.assign(mocks.user, { role: "candidate" });
});

describe("JobPostingDetail in Vietnamese", () => {
  it("translates the labels around a posting and leaves its text alone", async () => {
    mocks.get.mockResolvedValue(detail());
    mount();
    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Go Backend Engineer",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Quay lại bảng tin tuyển dụng" }),
    ).toBeInTheDocument();

    // Data from the API.
    expect(
      screen.getByText("We build reliable APIs in Go."),
    ).toBeInTheDocument();
    expect(screen.getByText("Hanoi")).toBeInTheDocument();
    expect(screen.getByText("Full-time")).toBeInTheDocument();
    expect(screen.getByText(/Acme/)).toBeInTheDocument();
    const required = screen.getByRole("list", { name: "Bắt buộc" });
    expect(within(required).getByText("Go")).toHaveAttribute(
      "title",
      "Ngôn ngữ lập trình",
    );
    const nice = screen.getByRole("list", { name: "Nên có" });
    // An unknown target is shown as words, not translated.
    expect(within(nice).getByText("Kubernetes")).toHaveAttribute(
      "title",
      "Cloud platform",
    );

    // Chrome.
    expect(
      screen.getByRole("button", { name: "Ứng tuyển bằng CV của bạn" }),
    ).toBeInTheDocument();
    for (const label of [
      "Cấp bậc",
      "Loại hình công việc",
      "Mức lương",
      "Thời lượng phỏng vấn",
    ])
      expect(screen.getByText(label)).toBeInTheDocument();
    expect(screen.getByText("Middle")).toBeInTheDocument();
    expect(screen.getByText("10.000.000 – 20.000.000")).toBeInTheDocument();
    expect(screen.getByText("30 phút")).toBeInTheDocument();
    expect(screen.getByText(/mở trong tab mới/)).toBeInTheDocument();
    for (const heading of [
      "Kỹ năng",
      "Quy trình tuyển dụng",
      "Mô tả công việc",
    ])
      expect(
        screen.getByRole("heading", { name: heading }),
      ).toBeInTheDocument();
    expect(screen.getByText("Tạo ngày 12 thg 10, 2026")).toBeInTheDocument();
  });

  it("translates the skill expander", async () => {
    mocks.get.mockResolvedValue(detail({ requirements: skills }));
    mount();
    const showAll = await screen.findByRole("button", {
      name: "Xem tất cả 12 kỹ năng",
    });
    fireEvent.click(showAll);
    expect(
      screen.getByRole("button", { name: "Thu gọn danh sách kỹ năng" }),
    ).toBeInTheDocument();
  });

  it("translates the applied state", async () => {
    mocks.get.mockResolvedValue(detail({ already_applied: true }));
    mount();
    expect(
      await screen.findByRole("link", {
        name: "Đã ứng tuyển · Xem đơn ứng tuyển",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Bạn đã ứng tuyển vào tin tuyển dụng này."),
    ).toBeInTheDocument();
  });

  it("explains the invalid, unavailable and failed states without the server message", async () => {
    const first = mount("not-a-uuid");
    expect(
      screen.getByRole("heading", { name: "Liên kết này không hợp lệ" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Xem các tin tuyển dụng đang mở" }),
    ).toBeInTheDocument();
    first.unmount();

    mocks.get.mockRejectedValue(failure(404));
    const second = mount();
    expect(
      await screen.findByRole("heading", {
        name: "Tin tuyển dụng này không còn khả dụng",
      }),
    ).toBeInTheDocument();
    second.unmount();

    mocks.get.mockRejectedValue(failure(500));
    mount();
    expect(
      await screen.findByRole("heading", {
        name: "Không thể tải tin tuyển dụng này",
      }),
    ).toBeInTheDocument();
    expect(screen.queryByText("server says")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Thử lại" })).toBeInTheDocument();
  });

  it("translates the long description toggle", async () => {
    mocks.get.mockResolvedValue(
      detail({ raw_text: "Line of text. ".repeat(80) }),
    );
    mount();
    fireEvent.click(
      await screen.findByRole("button", { name: "Xem toàn bộ mô tả" }),
    );
    expect(screen.getByRole("button", { name: "Thu gọn" })).toBeInTheDocument();
  });
});
