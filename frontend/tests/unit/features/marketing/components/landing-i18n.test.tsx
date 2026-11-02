import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { expect, it, vi } from "vitest";
import { FaqSection } from "@/features/marketing/components/faq-section";
import { RoleCueLanding } from "@/features/marketing/components/rolecue-landing";
import { WorkWheelSection } from "@/features/marketing/components/work-wheel-section";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ refresh: vi.fn() }),
}));

it("renders the landing hero, navigation and footer in Vietnamese", () => {
  renderWithIntl(<RoleCueLanding />, { locale: "vi" });

  expect(
    screen.getByRole("heading", { level: 1, name: "Sẵn sàng bước vào." }),
  ).toBeInTheDocument();
  const cta = screen.getByRole("link", { name: "Bắt đầu ngay" });
  expect(cta).toHaveAttribute("href", "/interviews/new/job-description");
  expect(
    screen.getByText("Luyện đúng vị trí trước khi bước vào phòng."),
  ).toBeInTheDocument();
  expect(screen.getByText("Luyện phỏng vấn như thật")).toBeInTheDocument();
  expect(screen.getByText("THEO VỊ TRÍ")).toBeInTheDocument();
  expect(screen.getByRole("region", { name: "Phần đầu trang RoleCue" }));

  const nav = screen.getByRole("navigation", { name: "Điều hướng chính" });
  expect(within(nav).getByRole("link", { name: "Tính năng" })).toHaveAttribute(
    "href",
    "#about",
  );
  expect(
    screen.getByRole("button", { name: "Mở điều hướng" }),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("navigation", { name: "Điều hướng cuối trang" }),
  ).toBeInTheDocument();

  // The feature carousel, closing invitation and its accessible names.
  expect(
    screen.getByRole("heading", {
      name: /Phòng luyện tập.*vị trí sắp tới\./,
    }),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("tab", { name: /Bối cảnh từ JD/ }),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("button", { name: "Tính năng tiếp theo" }),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("link", { name: "Bắt đầu miễn phí" }),
  ).toHaveAttribute("href", "/register");
  expect(
    screen.getByRole("img", {
      name: "Một ứng viên xem bản mô tả vị trí mục tiêu bên chiếc bàn làm việc xanh dịu, đủ ánh sáng",
    }),
  ).toBeInTheDocument();
});

it("keeps the English landing copy unchanged", () => {
  renderWithIntl(<RoleCueLanding />);

  expect(
    screen.getByRole("heading", { level: 1, name: "Walk in ready." }),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("link", { name: "Start for free" }),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("heading", {
      name: /Your next interview\s*starts here\./,
    }),
  ).toBeInTheDocument();
  expect(screen.getByRole("tab", { name: /Voice-first conversations/ }));
});

it("renders the practice loop in Vietnamese", () => {
  const width = vi
    .spyOn(HTMLElement.prototype, "clientWidth", "get")
    .mockReturnValue(1440);
  const height = vi
    .spyOn(HTMLElement.prototype, "clientHeight", "get")
    .mockReturnValue(684);
  try {
    renderWithIntl(<WorkWheelSection />, { locale: "vi" });
    expect(
      screen.getByRole("heading", {
        name: "Nhìn gần hơn vào vòng luyện tập của RoleCue.",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("listbox", { name: "Vòng luyện tập" }),
    ).toBeInTheDocument();
    const choice = screen.getByRole("button", {
      name: "Xem: Thiết lập buổi luyện tập",
    });
    fireEvent.click(choice);
    expect(
      screen.getByRole("list", { name: "Chọn một khoảnh khắc luyện tập" }),
    ).toBeInTheDocument();
  } finally {
    width.mockRestore();
    height.mockRestore();
  }
});

it("renders the FAQ in Vietnamese and opens an answer", async () => {
  renderWithIntl(<FaqSection />, { locale: "vi" });

  expect(
    screen.getByRole("heading", { name: "Vài điều nên biết." }),
  ).toBeInTheDocument();
  expect(screen.getByText("RoleCue dành cho ai?")).toBeInTheDocument();
  const question = screen.getByRole("button", {
    name: /Tôi có thể bắt đầu miễn phí không\?/,
  });
  fireEvent.click(question);
  await waitFor(() =>
    expect(
      screen.getByText(
        /Bạn có thể tạo tài khoản và bắt đầu luyện tập miễn phí/,
      ),
    ).toBeVisible(),
  );
});
