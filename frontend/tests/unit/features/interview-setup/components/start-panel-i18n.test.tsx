import { beforeEach, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import {
  StartPanel,
  type StartPlan,
} from "@/features/interview-setup/components/start-panel";
import type { MicrophoneReadiness } from "@/features/interview-setup/components/preflight-check";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
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
vi.mock("@/lib/api/client", () => ({
  apiClient: { get: mocks.get, post: mocks.post },
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}));
vi.mock("goey-toast", () => ({ gooeyToast: { info: vi.fn() } }));

const profileId = "22222222-2222-4222-8222-222222222222";
const practice: StartPlan = {
  kind: "new",
  request: { type: "practice", roleProfileId: profileId },
};

function wallet(balance: number) {
  mocks.get.mockImplementation((url: string) =>
    url === "/wallet"
      ? Promise.resolve({
          data: {
            success: true,
            data: { id: "w", user_id: mocks.user.id, balance },
          },
        })
      : Promise.reject(new Error(`unexpected GET ${url}`)),
  );
}

function renderPanel(
  readiness: MicrophoneReadiness = "ready",
  plan: StartPlan = practice,
) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return renderWithIntl(
    <QueryClientProvider client={client}>
      <StartPanel
        plan={plan}
        readiness={readiness}
        links={{ roleProfileId: profileId }}
        returnPath="/interviews/new/preflight"
      />
    </QueryClientProvider>,
    { locale: "vi" },
  );
}

beforeEach(() => {
  mocks.get.mockReset();
  mocks.post.mockReset();
});

it("shows the cost in xu, the balance, and a Vietnamese start button", async () => {
  wallet(1500);
  renderPanel();
  expect(
    screen.getByRole("heading", { name: "Phỏng vấn luyện tập" }),
  ).toBeInTheDocument();
  expect(
    screen.getByText(
      "5 xu sẽ được trừ một lần khi cuộc phỏng vấn bắt đầu. Tiếp tục lại sau đó là miễn phí.",
    ),
  ).toBeInTheDocument();
  expect(screen.getByText("Giá")).toBeInTheDocument();
  expect(await screen.findByText("1.500 xu")).toBeInTheDocument();
  expect(
    screen.getByRole("button", { name: "Bắt đầu phỏng vấn · 5 xu" }),
  ).toBeEnabled();
});

it("explains a missing microphone and a short balance in Vietnamese", async () => {
  wallet(3);
  renderPanel("muted");
  expect(await screen.findByText(/Bạn cần thêm/)).toHaveTextContent(
    "Bạn cần thêm 2 xu để bắt đầu.",
  );
  expect(
    screen.getByRole("button", { name: "Nạp thêm xu" }),
  ).toBeInTheDocument();
  expect(
    screen.getByText("Hãy bật tiếng micro để bắt đầu."),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("button", { name: "Bắt đầu phỏng vấn · 5 xu" }),
  ).toBeDisabled();
});

it("translates a refused start by its code, never the server text", async () => {
  wallet(9);
  mocks.post.mockRejectedValue(
    new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
      status: 400,
      statusText: "Error",
      data: {
        success: false,
        error: { code: "VALIDATION_ERROR", message: "avatar not found" },
      },
      headers: new AxiosHeaders(),
      config: { headers: new AxiosHeaders() },
    }),
  );
  renderPanel();
  await screen.findByText("9 xu");
  fireEvent.click(screen.getByRole("button", { name: /Bắt đầu phỏng vấn/ }));
  await waitFor(() =>
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Avatar này không còn trong kho avatar của bạn. Hãy chọn avatar khác.",
    ),
  );
  expect(screen.queryByText(/avatar not found/)).not.toBeInTheDocument();
  expect(
    screen.getByRole("link", { name: "Đổi người phỏng vấn" }),
  ).toBeVisible();
});
