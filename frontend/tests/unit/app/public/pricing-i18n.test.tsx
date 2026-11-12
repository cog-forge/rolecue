import { createTranslator } from "next-intl";
import { renderWithIntl, messagesFor } from "@tests/helpers/intl";
import { screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({ useRouter: () => ({ refresh: vi.fn() }) }));

vi.mock("next-intl/server", () => ({
  getTranslations: async (namespace: string) =>
    createTranslator({
      locale: "vi",
      messages: messagesFor("vi"),
      namespace: namespace as "public.pages",
    }),
}));

it("renders translated public navigation to all information pages", async () => {
  const { PublicPageShell } =
    await import("@/features/marketing/components/public-page-shell");
  renderWithIntl(await PublicPageShell({ children: <h1>Test</h1> }), {
    locale: "vi",
  });
  expect(screen.getAllByRole("link", { name: "Bảng giá" })[0]).toHaveAttribute(
    "href",
    "/pricing",
  );
  expect(screen.getAllByRole("link", { name: "Bảo mật" })[0]).toHaveAttribute(
    "href",
    "/privacy",
  );
  expect(
    screen.getAllByRole("link", { name: "Điều khoản" })[0],
  ).toHaveAttribute("href", "/terms");
});
