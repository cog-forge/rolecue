import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { JobBoard } from "@/features/job-board/components/job-board";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  detail: null as null | ((id: string) => Promise<unknown>),
  search: new URLSearchParams(),
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
vi.mock("@/lib/api/client", () => ({
  apiClient: {
    get: (path: string, options: { params?: Record<string, unknown> }) =>
      mocks.detail && path !== "/job-board"
        ? mocks.detail(path.split("/").pop() ?? "")
        : mocks.get(path, options),
  },
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));
vi.mock("next/navigation", () => ({
  useSearchParams: () => mocks.search,
}));

const posting = (extra: Record<string, unknown> = {}) => ({
  id: "33333333-3333-4333-8333-000000000000",
  title: "Senior Go Engineer",
  company_name: "Acme Corp",
  location: "Hanoi",
  employment_type: "Full-time",
  salary_min: 10_000_000,
  salary_max: 20_000_000,
  seniority_level: "senior",
  created_at: "2026-10-12T08:00:00Z",
  top_skills: ["Go", "Kafka"],
  required_skill_count: 5,
  ...extra,
});
const page = (items: ReturnType<typeof posting>[], total = items.length) => ({
  data: {
    success: true,
    data: { items, pagination: { limit: 20, offset: 0, total } },
  },
});

function mount(search = "") {
  mocks.search = new URLSearchParams(search);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  client.setQueryData(["auth", "me"], mocks.user);
  return renderWithIntl(
    <QueryClientProvider client={client}>
      <JobBoard />
    </QueryClientProvider>,
    { locale: "vi" },
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
});

describe("JobBoard in Vietnamese", () => {
  it("translates the page chrome, filters and card labels but not the posting data", async () => {
    mocks.get.mockResolvedValue(page([posting()]));
    mount();

    expect(
      screen.getByRole("heading", { level: 1, name: "Bảng tin tuyển dụng" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent(
      "Đang tải các tin tuyển dụng đang mở…",
    );

    const list = await screen.findByRole("list", {
      name: "Tin tuyển dụng đang mở, 1–1 trên 1",
    });
    const search = screen.getByRole("search", {
      name: "Tìm tin tuyển dụng đang mở",
    });
    expect(within(search).getByLabelText("Chức danh")).toHaveAttribute(
      "placeholder",
      "VD: Backend Engineer",
    );
    expect(within(search).getByLabelText("Địa điểm")).toBeInTheDocument();
    expect(
      within(search).getByLabelText("Loại hình công việc"),
    ).toBeInTheDocument();
    expect(
      within(search).getByText("Khớp chính xác, VD: Full-time"),
    ).toBeInTheDocument();
    expect(
      within(search).getByRole("button", { name: "Tìm kiếm" }),
    ).toBeInTheDocument();

    // Posting data renders exactly as the API sent it.
    const [card] = within(list)
      .getAllByRole("listitem")
      .filter((item) => item.parentElement === list);
    expect(
      within(card).getByRole("link", { name: "Senior Go Engineer" }),
    ).toBeInTheDocument();
    expect(card).toHaveTextContent("Acme Corp");
    expect(card).toHaveTextContent("Hanoi");
    expect(card).toHaveTextContent("Full-time");
    // Labels and numbers follow the language.
    expect(card).toHaveTextContent("Senior");
    expect(card).toHaveTextContent("10.000.000 – 20.000.000");
    expect(card).toHaveTextContent("Tạo ngày 12 thg 10, 2026");
    const skills = within(card).getByRole("list", { name: "Kỹ năng chính" });
    expect(
      within(skills)
        .getAllByRole("listitem")
        .map((item) => item.textContent),
    ).toEqual(["Go", "Kafka", "+3 kỹ năng nữa"]);
    expect(screen.getByText("Hiển thị 1–1 trên 1")).toBeInTheDocument();
    expect(
      screen.getByRole("navigation", { name: "Phân trang" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Trước" })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
    expect(screen.getByRole("link", { name: "Sau" })).toBeInTheDocument();
  });

  it("maps the seniority level and leaves an unknown employment type as stored", async () => {
    mocks.get.mockResolvedValue(
      page([
        posting({
          seniority_level: "intern",
          employment_type: "Hợp đồng thời vụ",
          salary_min: 5_000_000,
          salary_max: null,
          published_at: "2026-10-12T08:00:00Z",
        }),
      ]),
    );
    mount();
    const card = (
      await screen.findByRole("link", { name: "Senior Go Engineer" })
    ).closest("li") as HTMLElement;
    expect(card).toHaveTextContent("Thực tập sinh");
    expect(card).toHaveTextContent("Hợp đồng thời vụ");
    expect(card).toHaveTextContent("Từ 5.000.000");
    expect(card).toHaveTextContent(/Đăng /);
  });

  it("explains an empty board and an empty search in Vietnamese", async () => {
    mocks.get.mockResolvedValue(page([]));
    const first = mount();
    expect(
      await screen.findByText("Hiện chưa có tin tuyển dụng nào đang mở."),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Hãy quay lại sau: tin tuyển dụng mới sẽ xuất hiện sau khi được duyệt và nạp quỹ.",
      ),
    ).toBeInTheDocument();
    first.unmount();

    mount("type=Contract");
    expect(
      await screen.findByText(
        "Không có tin tuyển dụng nào đang mở khớp với tìm kiếm của bạn.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Xóa tìm kiếm" })).toHaveAttribute(
      "href",
      "/jobs",
    );
    expect(screen.getByRole("link", { name: "Xóa" })).toBeInTheDocument();
  });

  it("shows the past-the-end and error states in Vietnamese", async () => {
    mocks.get.mockResolvedValue(page([], 30));
    const first = mount("page=5");
    expect(
      await screen.findByText("Trang này nằm ngoài danh sách."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Về trang đầu tiên" }),
    ).toBeInTheDocument();
    first.unmount();

    mocks.get.mockRejectedValue(new Error("boom"));
    mount();
    expect(
      await screen.findByRole("heading", {
        name: "Không thể tải bảng tin tuyển dụng",
      }),
    ).toBeInTheDocument();
    expect(screen.queryByText("boom")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Thử lại" })).toBeInTheDocument();
  });

  describe("on a wide screen", () => {
    const original = window.matchMedia;
    beforeEach(() => {
      window.matchMedia = (query: string) =>
        ({
          ...original(query),
          matches: query === "(min-width: 1024px)",
        }) as MediaQueryList;
      mocks.detail = vi.fn(() =>
        Promise.resolve({
          data: {
            success: true,
            data: {
              ...posting(),
              company_website: null,
              interview_duration: 30,
              raw_text: "Build reliable APIs.",
              requirements: [],
              already_applied: false,
            },
          },
        }),
      );
    });
    afterEach(() => {
      window.matchMedia = original;
      mocks.detail = null;
    });

    it("labels the selected posting panel", async () => {
      mocks.get.mockResolvedValue(page([posting()]));
      mount();
      const panel = await screen.findByRole("complementary", {
        name: "Tin tuyển dụng đang chọn",
      });
      expect(
        await within(panel).findByText("Build reliable APIs."),
      ).toBeInTheDocument();
      expect(
        within(panel).getByRole("button", {
          name: "Ứng tuyển bằng CV của bạn",
        }),
      ).toBeInTheDocument();
    });
  });
});
