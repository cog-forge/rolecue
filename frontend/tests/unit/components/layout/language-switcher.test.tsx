import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen } from "@testing-library/react";
import { LanguageSwitcher } from "@/components/layout/language-switcher";
import { renderWithIntl } from "@tests/helpers/intl";

const mocks = vi.hoisted(() => ({ refresh: vi.fn() }));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ refresh: mocks.refresh }),
}));

function clearLocaleCookie() {
  document.cookie = "NEXT_LOCALE=; Path=/; Max-Age=0";
}

function openMenu(name: RegExp) {
  fireEvent.keyDown(screen.getByRole("button", { name }), { key: "Enter" });
}

describe("LanguageSwitcher", () => {
  beforeEach(clearLocaleCookie);
  afterEach(clearLocaleCookie);

  it("shows the active language by its own name and offers both languages by their own names", async () => {
    renderWithIntl(<LanguageSwitcher />);
    expect(
      screen.getByRole("button", { name: "Change language: English" }),
    ).toHaveTextContent("English");

    openMenu(/change language/i);
    const options = await screen.findAllByRole("menuitemradio");
    expect(options.map((option) => option.textContent)).toEqual([
      "English",
      "Tiếng Việt",
    ]);
    expect(
      screen.getByRole("menuitemradio", { name: "English" }),
    ).toBeChecked();
  });

  it("saves the choice in a cookie and refreshes the page in place", async () => {
    renderWithIntl(<LanguageSwitcher />);
    openMenu(/change language/i);
    fireEvent.click(
      await screen.findByRole("menuitemradio", { name: "Tiếng Việt" }),
    );

    expect(document.cookie).toContain("NEXT_LOCALE=vi");
    expect(mocks.refresh).toHaveBeenCalledTimes(1);
  });

  it("does nothing when the active language is chosen again", async () => {
    renderWithIntl(<LanguageSwitcher />, { locale: "vi" });
    openMenu(/đổi ngôn ngữ/i);
    fireEvent.click(
      await screen.findByRole("menuitemradio", { name: "Tiếng Việt" }),
    );

    expect(document.cookie).not.toContain("NEXT_LOCALE");
    expect(mocks.refresh).not.toHaveBeenCalled();
  });

  it("is labelled in the active language", () => {
    renderWithIntl(<LanguageSwitcher />, { locale: "vi" });
    expect(
      screen.getByRole("button", { name: "Đổi ngôn ngữ: Tiếng Việt" }),
    ).toBeInTheDocument();
  });

  it("can render icon-only while keeping its accessible name", () => {
    renderWithIntl(<LanguageSwitcher showName={false} />);
    const button = screen.getByRole("button", {
      name: "Change language: English",
    });
    expect(button).not.toHaveTextContent("English");
  });
});
