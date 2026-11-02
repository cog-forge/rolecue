import { beforeEach, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RoleProfilePicker } from "@/features/interview-setup/components/role-profile-picker";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  user: {
    id: "11111111-1111-4111-8111-111111111111",
    full_name: "Cam Candidate",
    email: "cam@example.com",
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
  useRouter: () => ({ replace: vi.fn() }),
}));

const backend = "22222222-2222-4222-8222-000000000002";
const sessionId = "66666666-6666-4666-8666-666666666666";

const summary = (
  id: string,
  title: string,
  seniorityLevel: string | null = "mid",
) => ({
  id,
  title,
  seniorityLevel,
  status: "confirmed",
  createdAt: "2026-10-20T08:00:00Z",
});

function api(unfinished = false) {
  mocks.get.mockImplementation((url: string) => {
    if (url === "/role-profiles")
      return Promise.resolve({
        data: {
          success: true,
          data: {
            items: [
              summary(backend, "Backend Engineer – Payments"),
              summary("22222222-2222-4222-8222-000000000009", "Data", null),
            ],
            pagination: { limit: 20, offset: 0, total: 2 },
          },
        },
      });
    if (url === "/interviews")
      return Promise.resolve({
        data: {
          success: true,
          data: {
            items: unfinished
              ? [
                  {
                    id: sessionId,
                    type: "practice",
                    started_at: "2026-10-28T08:00:00Z",
                    completed_at: null,
                    evaluation_status: "pending",
                    status: "in_progress",
                    score: null,
                    role_profile_title: "Backend Engineer – Payments",
                    job_posting_title: null,
                    application_id: null,
                  },
                ]
              : [],
            total: unfinished ? 1 : 0,
            page: 1,
            page_size: 100,
          },
        },
      });
    if (url === `/interviews/${sessionId}`)
      return Promise.resolve({
        data: {
          success: true,
          data: {
            id: sessionId,
            type: "practice",
            role_profile_id: backend,
            application_id: null,
            avatar_id: null,
            voice_id: null,
            started_at: "2026-10-28T08:00:00Z",
            completed_at: null,
            evaluation_status: "pending",
            question_count: 8,
            answered_count: 3,
          },
        },
      });
    return Promise.reject(new Error(`unexpected GET ${url}`));
  });
}

function renderPicker(roleProfileId?: string) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return renderWithIntl(
    <QueryClientProvider client={client}>
      <RoleProfilePicker roleProfileId={roleProfileId} />
    </QueryClientProvider>,
    { locale: "vi" },
  );
}

beforeEach(() => {
  mocks.get.mockReset();
});

it("speaks Vietnamese and leaves the Role Profile titles as stored", async () => {
  api();
  renderPicker();
  expect(
    await screen.findByRole("heading", { level: 1, name: "Chọn hồ sơ vị trí" }),
  ).toBeInTheDocument();
  expect(screen.getByText("Thiết lập phỏng vấn")).toBeInTheDocument();
  const group = await screen.findByRole("radiogroup", {
    name: "Các hồ sơ vị trí đã xác nhận",
  });
  const first = within(group).getByRole("radio", {
    name: /Backend Engineer – Payments/,
  });
  expect(first).toBeInTheDocument();
  // The raw seniority enum becomes a label, the missing one a sentence; the date is Vietnamese.
  expect(within(group).getByText("Middle")).toBeInTheDocument();
  expect(within(group).getByText("Chưa đặt cấp độ")).toBeInTheDocument();
  expect(group).toHaveTextContent("Tạo ngày 20 thg 10, 2026");
  expect(
    screen.getByRole("button", { name: /Tiếp tục chọn người phỏng vấn/ }),
  ).toBeDisabled();
  expect(
    screen.getByText("Hãy chọn một hồ sơ vị trí để tiếp tục."),
  ).toBeInTheDocument();
  expect(screen.getByRole("link", { name: /Thêm hồ sơ vị trí/ })).toBeVisible();
});

it("shows the free resume path in Vietnamese with the progress count", async () => {
  api(true);
  renderPicker(backend);
  expect(
    await screen.findByText(
      "Bạn có một cuộc phỏng vấn chưa hoàn thành cho vị trí này",
    ),
  ).toBeInTheDocument();
  expect(screen.getByText(/Đã trả lời/)).toHaveTextContent("3/8");
  expect(
    screen.getByText(
      "Tiếp tục hoàn toàn miễn phí và giữ nguyên người phỏng vấn cùng giọng nói.",
    ),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("link", { name: /Tiếp tục phỏng vấn/ }),
  ).toHaveAttribute(
    "href",
    `/interviews/new/preflight?roleProfileId=${backend}`,
  );
});
