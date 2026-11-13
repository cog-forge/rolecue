import { fireEvent, screen } from "@testing-library/react";
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

  it("filters the real 404 destinations without navigating", () => {
    renderWithIntl(<NotFound />, { locale: "en" });
    fireEvent.change(screen.getByRole("searchbox", { name: "Search pages" }), {
      target: { value: "pricing" },
    });
    expect(screen.getByRole("link", { name: "Pricing" })).toHaveAttribute(
      "href",
      "/pricing",
    );
    expect(
      screen.queryByRole("link", { name: "Return home" }),
    ).not.toBeInTheDocument();
    fireEvent.change(screen.getByRole("searchbox", { name: "Search pages" }), {
      target: { value: "nothing matches" },
    });
    expect(screen.getByRole("status")).toHaveTextContent(
      "No pages match your search.",
    );
  });

  it("renders workspace errors in Vietnamese", () => {
    renderWithIntl(
      <WorkspaceError
        error={new Error("private technical detail")}
        reset={vi.fn()}
      />,
      {
        locale: "vi",
      },
    );
    expect(
      screen.getByRole("heading", { name: "Không thể tải trang này" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Thử lại" })).toBeInTheDocument();
    expect(
      screen.queryByText("private technical detail"),
    ).not.toBeInTheDocument();
  });

  it("announces loading in Vietnamese", () => {
    renderWithIntl(<Loading />, { locale: "vi" });
    expect(screen.getByRole("status")).toHaveTextContent("Đang tải trang");
  });
});
