import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";
import { Flag } from "@/components/ui/flags";
import { localeFlags, locales } from "@/lib/i18n/config";

describe("Flag", () => {
  it("draws an inline SVG that is hidden from assistive technology", () => {
    const { container } = render(<Flag code="vn" />);
    const flag = container.querySelector("[data-flag='vn']");
    expect(flag?.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
  });

  it("draws a distinct flag per code", () => {
    const { container } = render(
      <>
        <Flag code="vn" />
        <Flag code="gb" />
      </>,
    );
    const [vn, gb] = Array.from(container.querySelectorAll("svg"));
    expect(vn.innerHTML).not.toBe(gb.innerHTML);
  });

  it("gives every supported locale a flag", () => {
    for (const locale of locales) expect(localeFlags[locale]).toBeTruthy();
  });
});
