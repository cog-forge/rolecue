import { describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderWithIntl } from "@tests/helpers/intl";
import { RoleProfileEntry } from "@/features/role-profile/components/role-profile-entry";

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => ({ id: "11111111-1111-4111-8111-111111111111" }),
}));
vi.mock("@/lib/api/client", () => ({ apiClient: { post: vi.fn() } }));

function renderEntry(locale: "en" | "vi") {
  return renderWithIntl(
    <QueryClientProvider client={new QueryClient()}>
      <RoleProfileEntry />
    </QueryClientProvider>,
    { locale },
  );
}

describe("RoleProfileEntry in Vietnamese", () => {
  it("translates the entry screen", () => {
    renderEntry("vi");
    expect(
      screen.getByRole("heading", {
        level: 1,
        name: "Đưa vị trí vào phòng phỏng vấn.",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("list", { name: "Tiến trình hồ sơ vị trí" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("tab", { name: "Dán văn bản" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("tab", { name: "Tải lên PDF" }),
    ).toBeInTheDocument();
    expect(
      screen.getByLabelText(/Nội dung mô tả công việc/),
    ).toBeInTheDocument();
    expect(screen.getByText("0 / 20.000")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Phân tích vị trí" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Hồ sơ vị trí của bạn được giữ riêng tư."),
    ).toBeInTheDocument();
    expect(screen.getByText("Điều gì xảy ra tiếp theo")).toBeInTheDocument();
  });

  it("validates the pasted text in Vietnamese and keeps the typed JD as is", async () => {
    renderEntry("vi");
    const box = screen.getByLabelText(/Nội dung mô tả công việc/);
    fireEvent.change(box, { target: { value: "Quá ngắn <b>JD</b>" } });
    fireEvent.click(screen.getByRole("button", { name: "Phân tích vị trí" }));
    await waitFor(() =>
      expect(
        screen.getByText(
          "Hãy dán từ 100 đến 20.000 ký tự văn bản mô tả công việc.",
        ),
      ).toBeInTheDocument(),
    );
    expect(box).toHaveValue("Quá ngắn <b>JD</b>");
  });

  it("keeps the English wording in English", async () => {
    renderEntry("en");
    expect(
      screen.getByRole("heading", { name: "Bring the role into the room." }),
    ).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText(/Job description text/), {
      target: { value: "Too short" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Analyze role" }));
    expect(
      await screen.findByText(
        "Paste 100–20,000 characters of job description text.",
      ),
    ).toBeInTheDocument();
  });
});
