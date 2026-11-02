import { describe, expect, it } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { RoleProfileStatus } from "@/features/role-profile/components/role-profile-status";

describe("RoleProfileStatus", () => {
  it.each([
    ["draft", "Bản nháp", "Draft"],
    ["analyzing", "Đang phân tích", "Analyzing"],
    ["review", "Cần xem lại", "Needs review"],
    ["confirmed", "Đã xác nhận", "Confirmed"],
  ] as const)("maps %s to a label per language", (status, vi, en) => {
    const { unmount } = renderWithIntl(<RoleProfileStatus status={status} />, {
      locale: "vi",
    });
    expect(screen.getByText(vi)).toBeInTheDocument();
    unmount();
    renderWithIntl(<RoleProfileStatus status={status} />);
    expect(screen.getByText(en)).toBeInTheDocument();
  });
});
