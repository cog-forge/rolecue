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

  it("shows only the flag of the active language on the button and each language by flag and name in the menu", async () => {
    renderWithIntl(<LanguageSwitcher />);
    const button = screen.getByRole("button", {
      name: "Change language: English",
    });
    expect(button.textContent).toBe("");
    expect(button.querySelector("[data-flag='gb']")).not.toBeNull();
    expect(button.querySelector("[data-flag='vn']")).toBeNull();

    openMenu(/change language/i);
    const options = await screen.findAllByRole("menuitemradio");
    expect(options.map((option) => option.textContent)).toEqual([
      "English",
      "Tiếng Việt",
    ]);
    expect(options[0].querySelector("[data-flag='gb']")).not.toBeNull();
    expect(options[1].querySelector("[data-flag='vn']")).not.toBeNull();
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

  it("shows the Vietnam flag while Vietnamese is active", () => {
    renderWithIntl(<LanguageSwitcher />, { locale: "vi" });
    const button = screen.getByRole("button", {
      name: "Đổi ngôn ngữ: Tiếng Việt",
    });
    expect(button.querySelector("[data-flag='vn']")).not.toBeNull();
    expect(button.textContent).toBe("");
  });

  it("keeps the flags out of the accessibility tree", async () => {
    renderWithIntl(<LanguageSwitcher />);
    openMenu(/change language/i);
    await screen.findAllByRole("menuitemradio");
    for (const svg of document.querySelectorAll("[data-flag] svg"))
      expect(svg).toHaveAttribute("aria-hidden", "true");
  });

  it("keeps a 44px touch target", () => {
    renderWithIntl(<LanguageSwitcher />);
    expect(
      screen.getByRole("button", { name: /change language/i }),
    ).toHaveClass("h-11", "min-w-11");
  });
});
