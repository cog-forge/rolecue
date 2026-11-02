import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import NotFound from "@/app/not-found";
import WorkspaceError from "@/app/(workspace)/error";
import Loading from "@/app/(workspace)/loading";
import { renderWithIntl } from "../../helpers/intl";

describe("shared page states", () => {
  it("renders the not-found page in Vietnamese", () => {
    renderWithIntl(<NotFound />, { locale: "vi" });
    expect(
      screen.getByRole("heading", { name: "Không tìm thấy trang" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Về trang chủ" })).toHaveAttribute(
      "href",
      "/",
    );
  });

  it("renders workspace errors in Vietnamese", () => {
    renderWithIntl(<WorkspaceError error={new Error()} reset={vi.fn()} />, {
      locale: "vi",
    });
    expect(
      screen.getByRole("heading", { name: "Không thể tải trang này" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Thử lại" })).toBeInTheDocument();
  });

  it("announces loading in Vietnamese", () => {
    renderWithIntl(<Loading />, { locale: "vi" });
    expect(screen.getByRole("status")).toHaveTextContent("Đang tải trang");
  });
});
