import { act, fireEvent, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { StatePreview } from "@/app/state-preview/state-preview";
import { renderWithIntl } from "../../helpers/intl";

afterEach(() => vi.useRealTimers());
describe("state preview", () => {
  it("keeps loading visible and only retries when requested", () => {
    vi.useFakeTimers();
    renderWithIntl(<StatePreview initialView="Table loading" />, {
      locale: "en",
    });
    act(() => vi.advanceTimersByTime(10000));
    expect(
      screen.getByRole("button", { name: "Bảng đang tải" }),
    ).toHaveAttribute("aria-pressed", "true");
    fireEvent.click(screen.getByRole("button", { name: "Bảng tải lỗi" }));
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    act(() => vi.advanceTimersByTime(2000));
    expect(
      screen.getByRole("table", { name: "Example table" }),
    ).toHaveTextContent("Backend Engineer");
  });
  it("cancels a pending retry when another state is selected", () => {
    vi.useFakeTimers();
    renderWithIntl(<StatePreview initialView="Page error" />, { locale: "en" });
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    fireEvent.click(screen.getByRole("button", { name: "Bảng trống" }));
    act(() => vi.advanceTimersByTime(2000));
    expect(
      screen.getByRole("heading", { name: "No items found" }),
    ).toBeVisible();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });
});
