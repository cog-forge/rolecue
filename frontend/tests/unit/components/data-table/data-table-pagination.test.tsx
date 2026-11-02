import { describe, expect, it, vi } from "vitest";
import { fireEvent, screen } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { DataTablePagination } from "@/components/data-table/data-table-pagination";

function mount(locale: "en" | "vi", total = 45) {
  const onPageChange = vi.fn();
  renderWithIntl(
    <DataTablePagination
      offset={10}
      pageSize={10}
      total={total}
      disabled={false}
      onPageChange={onPageChange}
      label="Accounts"
    />,
    { locale },
  );
  return onPageChange;
}

describe("data table pagination", () => {
  it("keeps the English text", () => {
    const onPageChange = mount("en");
    expect(screen.getByRole("status")).toHaveTextContent(
      "11–20 of 45 · Page 2 of 5",
    );
    fireEvent.click(screen.getByRole("button", { name: "Next page" }));
    expect(onPageChange).toHaveBeenCalledWith(3);
    expect(screen.getByRole("button", { name: "Page 2" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(screen.getByLabelText("Jump to page")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Go" })).toBeInTheDocument();
  });

  it("translates the Vietnamese controls", () => {
    const onPageChange = mount("vi");
    expect(screen.getByRole("status")).toHaveTextContent(
      "11–20 trên 45 · Trang 2/5",
    );
    fireEvent.click(screen.getByRole("button", { name: "Trang cuối" }));
    expect(onPageChange).toHaveBeenCalledWith(5);
    expect(screen.getByRole("button", { name: "Trang trước" })).toBeEnabled();
    expect(screen.getByRole("form", { name: "Đi đến trang" })).toBeVisible();
    expect(screen.getByLabelText("Đến trang")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Đi" })).toBeDisabled();
  });

  it("translates the empty result", () => {
    mount("vi", 0);
    expect(screen.getByRole("status")).toHaveTextContent("0 kết quả");
  });
});
