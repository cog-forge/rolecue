import { isValidElement } from "react";
import { createTranslator } from "next-intl";
import { renderWithIntl, messagesFor } from "@tests/helpers/intl";
import { screen } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";

let activeLocale: "en" | "vi" = "vi";

vi.mock("next-intl/server", () => ({
  getTranslations: async (namespace: string) =>
    createTranslator({
      locale: activeLocale,
      messages: messagesFor(activeLocale),
      namespace: namespace as "public.pricing",
    }),
}));

beforeEach(() => {
  activeLocale = "vi";
});

it("renders the pricing placeholder in Vietnamese", async () => {
  const { default: PricingPage } = await import("@/app/(public)/pricing/page");
  renderWithIntl(await PricingPage(), { locale: "vi" });
  expect(
    screen.getByRole("heading", { level: 1, name: "Bảng giá" }),
  ).toBeInTheDocument();
  expect(
    screen.getByText(/Các gói xu phỏng vấn và thông tin gói dịch vụ/),
  ).toBeInTheDocument();
});

it("gives the pricing shell translated navigation and label", async () => {
  const { default: PricingLayout } =
    await import("@/app/(public)/pricing/layout");
  const element = await PricingLayout({ children: null });
  expect(isValidElement(element)).toBe(true);
  const props = (
    element as {
      props: { label: string; navigation: { label: string; href: string }[] };
    }
  ).props;
  expect(props.label).toBe("Công khai");
  expect(props.navigation).toEqual([
    { label: "Trang chủ", href: "/" },
    { label: "Bảng giá", href: "/pricing" },
    { label: "Đăng nhập", href: "/login" },
  ]);
});

it("keeps the English pricing copy unchanged", async () => {
  activeLocale = "en";
  const { default: PricingPage } = await import("@/app/(public)/pricing/page");
  renderWithIntl(await PricingPage());
  expect(
    screen.getByRole("heading", { level: 1, name: "Pricing" }),
  ).toBeInTheDocument();
  expect(
    screen.getByText(
      "Interview credit packages and plan information will be available once pricing is defined.",
    ),
  ).toBeInTheDocument();
});
