import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AvatarStudio } from "@/features/avatar/components/avatar-studio";
import { AvatarDetailsForm } from "@/features/avatar/components/avatar-details-form";
import { AvaturnCreator } from "@/features/avatar/components/avaturn-creator";
import {
  StudioJourney,
  StudioPhotoGuide,
} from "@/features/avatar/components/studio-journey";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  user: { id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", role: "candidate" },
  summary: { capacity: 3, used: 2, available: 1, is_full: false },
}));
vi.mock("@/lib/api/client", () => ({ apiClient: { get: mocks.get } }));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));
vi.mock("@/config/env", () => ({
  getAvaturnProjectUrl: () => "https://rolecue.avaturn.dev",
}));
// A creator that never finishes connecting: the chrome around the iframe is what is under test.
vi.mock("@avaturn/sdk", () => ({
  AvaturnSDK: class {
    on() {}
    init() {
      return new Promise<void>(() => {});
    }
    destroy() {}
  },
}));

beforeEach(() => {
  vi.clearAllMocks();
  mocks.summary = { capacity: 3, used: 2, available: 1, is_full: false };
  mocks.get.mockImplementation(async (url: string) => {
    if (url === "/inventory")
      return { data: { success: true, data: mocks.summary } };
    throw new Error(`unexpected GET ${url}`);
  });
});

function renderStudio() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return renderWithIntl(
    <QueryClientProvider client={client}>
      <AvatarStudio />
    </QueryClientProvider>,
    { locale: "vi" },
  );
}

describe("studio journey in Vietnamese", () => {
  it("names the steps and announces their state", () => {
    renderWithIntl(<StudioJourney current={1} complete={false} />, {
      locale: "vi",
    });
    const list = screen.getByRole("list", { name: "Tiến trình tạo avatar" });
    const steps = within(list).getAllByRole("listitem");
    expect(steps).toHaveLength(3);
    expect(steps[0]).toHaveTextContent("Thông tin, đã hoàn tất");
    expect(steps[0]).toHaveTextContent("Tên và tính cách");
    expect(steps[1]).toHaveTextContent("Tạo, bước hiện tại");
    expect(steps[1]).toHaveAttribute("aria-current", "step");
    expect(steps[2]).toHaveTextContent("Lưu, sắp tới");
  });

  it("translates the photo guide", () => {
    renderWithIntl(<StudioPhotoGuide />, { locale: "vi" });
    expect(
      screen.getByRole("complementary", { name: "Hướng dẫn chuẩn bị ảnh" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Chọn ánh sáng dịu và đều")).toBeInTheDocument();
    expect(
      screen.getByText(/Bạn sẽ đăng nhập Avaturn riêng/),
    ).toBeInTheDocument();
  });
});

describe("avatar details form in Vietnamese", () => {
  it("shows translated labels and validation, and keeps the typed text as is", async () => {
    const onStart = vi.fn().mockResolvedValue(undefined);
    renderWithIntl(<AvatarDetailsForm disabled={false} onStart={onStart} />, {
      locale: "vi",
    });
    expect(
      screen.getByRole("form", { name: "Thông tin avatar" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Bước 01 / Thông tin")).toBeInTheDocument();
    expect(screen.getByLabelText("Tên avatar")).toHaveAttribute(
      "placeholder",
      "Đặt tên cho người phỏng vấn của bạn",
    );
    const submit = screen.getByRole("button", { name: "Mở trình tạo Avaturn" });
    fireEvent.click(submit);
    expect(await screen.findByText("Vui lòng nhập tên.")).toBeInTheDocument();
    expect(screen.getByText("Vui lòng nhập tính cách.")).toBeInTheDocument();
    expect(onStart).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Hỗ trợ" }));
    expect(screen.getByLabelText("Tính cách")).toHaveValue(
      "Patient and encouraging. Ask clear questions and help me feel comfortable sharing my thought process.",
    );
    fireEvent.change(screen.getByLabelText("Tên avatar"), {
      target: { value: "Minh Thư" },
    });
    fireEvent.click(submit);
    await waitFor(() =>
      expect(onStart.mock.calls[0]?.[0]).toMatchObject({ name: "Minh Thư" }),
    );
  });

  it("limits the name length with a translated message", async () => {
    renderWithIntl(<AvatarDetailsForm disabled={false} onStart={vi.fn()} />, {
      locale: "vi",
    });
    fireEvent.change(screen.getByLabelText("Tên avatar"), {
      target: { value: "a".repeat(101) },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Mở trình tạo Avaturn" }),
    );
    expect(
      await screen.findByText(
        "Tên tối đa 100 ký tự và không được chứa ký tự null.",
      ),
    ).toBeInTheDocument();
  });
});

describe("Avaturn creator chrome in Vietnamese", () => {
  it("translates the toolbar and the restart confirmation", async () => {
    renderWithIntl(
      <AvaturnCreator
        url="https://rolecue.avaturn.dev"
        onExport={vi.fn()}
        expanded={false}
        onToggleExpanded={vi.fn()}
      />,
      { locale: "vi" },
    );
    expect(
      await screen.findByText("Đang kết nối với Avaturn…"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Mở rộng trình chỉnh sửa" }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Tải lại trình tạo" }));
    expect(
      await screen.findByRole("dialog", {
        name: "Khởi động lại trình tạo Avaturn?",
      }),
    ).toBeInTheDocument();
  });
});

describe("avatar studio in Vietnamese", () => {
  it("translates the page, the slot summary and the photo guide", async () => {
    renderStudio();
    expect(
      screen.getByRole("heading", { level: 1, name: "Tạo avatar" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("Đã dùng 2/3 ô avatar")).toBeInTheDocument();
    expect(
      screen.getByText("Còn 1 ô cho avatar tiếp theo của bạn"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Avatar Studio cá nhân/ }),
    ).toHaveAttribute("href", "/avatar-studio");
    expect(
      screen.getByRole("heading", {
        name: "Hãy tạo nét riêng cho người phỏng vấn của bạn.",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("list", { name: "Tiến trình tạo avatar" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Làm mới ô avatar" }),
    ).toBeInTheDocument();
  });

  it("explains a full inventory with a link back to it", async () => {
    mocks.summary = { capacity: 2, used: 2, available: 0, is_full: true };
    renderStudio();
    expect(
      await screen.findByText(
        "Kho avatar của bạn đã đầy. Hãy xóa một avatar hoặc mua thêm ô trước khi tạo avatar khác.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Quản lý kho avatar" }),
    ).toHaveAttribute("href", "/avatar-studio");
  });
});
