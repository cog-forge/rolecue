import { beforeEach, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { InterviewerPicker } from "@/features/interview-setup/components/interviewer-picker";

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
  useRouter: () => ({ push: vi.fn() }),
}));

const profileId = "22222222-2222-4222-8222-222222222222";
const mia = "44444444-4444-4444-8444-000000000001";
const ava = "55555555-5555-4555-8555-000000000001";

function api(avatars: { id: string; name: string }[]) {
  mocks.get.mockImplementation((url: string) => {
    if (url === "/inventory/avatars")
      return Promise.resolve({
        data: {
          success: true,
          data: {
            items: avatars.map((avatar) => ({
              ...avatar,
              personality: "Calm and precise; asks for trade-offs.",
              created_at: "2026-10-20T08:00:00Z",
              updated_at: "2026-10-20T08:00:00Z",
            })),
            pagination: { limit: 12, offset: 0, total: avatars.length },
          },
        },
      });
    if (url === "/voices")
      return Promise.resolve({
        data: {
          success: true,
          data: {
            items: [{ id: ava, name: "Ava - English" }],
            pagination: { limit: 100, offset: 0, total: 1 },
          },
        },
      });
    if (url === "/interviews")
      return Promise.resolve({
        data: {
          success: true,
          data: { items: [], total: 0, page: 1, page_size: 100 },
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
      <InterviewerPicker roleProfileId={roleProfileId} />
    </QueryClientProvider>,
    { locale: "vi" },
  );
}

beforeEach(() => {
  mocks.get.mockReset();
  vi.stubGlobal(
    "IntersectionObserver",
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
      takeRecords() {
        return [];
      }
    },
  );
});

it("translates the picker chrome and keeps avatar and voice names as data", async () => {
  api([{ id: mia, name: "Mia" }]);
  renderPicker(profileId);
  expect(
    await screen.findByRole("heading", {
      level: 1,
      name: "Chọn người phỏng vấn",
    }),
  ).toBeInTheDocument();
  expect(
    await screen.findByRole("radio", { name: /Người phỏng vấn 2D/ }),
  ).toBeChecked();
  // The avatar's name and personality are stored text, shown unchanged.
  expect(await screen.findByRole("radio", { name: /Mia/ })).toBeInTheDocument();
  expect(
    screen.getByText("Calm and precise; asks for trade-offs."),
  ).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Mở Avatar Studio" })).toBeVisible();
  expect(screen.getByRole("combobox", { name: "Giọng nói" })).toHaveTextContent(
    "Giọng mặc định",
  );
  expect(
    await screen.findByText("Chỉ hiển thị các giọng nói đang hoạt động."),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("link", { name: /Quay lại hồ sơ vị trí/ }),
  ).toBeVisible();
  expect(
    screen.getByRole("button", { name: /Tiếp tục kiểm tra trước/ }),
  ).toBeVisible();
});

it("asks for a Role Profile first, and invites an empty inventory to the Studio", async () => {
  api([]);
  const first = renderPicker();
  expect(
    await screen.findByRole("heading", {
      name: "Hãy chọn hồ sơ vị trí trước.",
    }),
  ).toBeInTheDocument();
  first.unmount();
  renderPicker(profileId);
  expect(
    await screen.findByRole("link", { name: "Tạo avatar trong Avatar Studio" }),
  ).toHaveAttribute("href", "/avatar-studio/new");
});
