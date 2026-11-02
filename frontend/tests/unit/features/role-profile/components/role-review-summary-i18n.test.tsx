import { describe, expect, it } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { RoleReviewSummary } from "@/features/role-profile/components/role-review-summary";
import { RoleProfileSteps } from "@/features/role-profile/components/role-profile-steps";
import type { RoleProfile } from "@/features/role-profile/api/contract";

const profile: RoleProfile = {
  id: "22222222-2222-4222-8222-222222222222",
  title: "Backend Engineer",
  rawText: "JD",
  hasOriginalDocument: false,
  seniorityLevel: "mid",
  requirements: [
    {
      name: "PostgreSQL",
      target: "Proficient",
      kind: "skill",
      category: "database",
      type: "preferred",
      evidenceText: "Experience with PostgreSQL.",
    },
    {
      name: "Kafka",
      target: "Tune consumers",
      kind: "technology",
      category: null,
      type: null,
      evidenceText: null,
    },
  ],
  refinementNote: "Skip front-end <topics>",
  status: "confirmed",
  createdAt: "2026-10-10T00:00:00Z",
};

describe("RoleReviewSummary in Vietnamese", () => {
  it("maps enums to labels and leaves stored and AI text untouched", () => {
    renderWithIntl(<RoleReviewSummary profile={profile} />, { locale: "vi" });
    expect(screen.getByText("Backend Engineer")).toBeInTheDocument();
    expect(screen.getByText("Mid-level")).toBeInTheDocument();
    expect(screen.getByText("PostgreSQL")).toBeInTheDocument();
    expect(
      screen.getByLabelText("Mức thành thạo cho PostgreSQL"),
    ).toHaveTextContent("Thành thạo");
    expect(screen.getByText("Cơ sở dữ liệu · Ưu tiên")).toBeInTheDocument();
    // A target saved in the earlier free-text format shows as written.
    expect(screen.getByText("Tune consumers")).toBeInTheDocument();
    expect(screen.getByLabelText("Mức thành thạo cho Kafka")).toHaveTextContent(
      "Mục tiêu đã lưu",
    );
    expect(screen.getByText("Skip front-end <topics>")).toBeInTheDocument();
    expect(screen.getByText("Ghi chú tinh chỉnh của bạn")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", {
        name: "Bản xem lại vị trí của bạn đã được lưu.",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Tiếp tục thiết lập/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Nguồn của yêu cầu 1" }),
    ).toBeInTheDocument();
  });
});

describe("RoleProfileSteps", () => {
  it("announces progress in the active language", () => {
    renderWithIntl(<RoleProfileSteps current="review" />, { locale: "vi" });
    const list = screen.getByRole("list", { name: "Tiến trình hồ sơ vị trí" });
    expect(list).toHaveTextContent("Thêm JD, đã hoàn tất");
    expect(list).toHaveTextContent("Xem lại vị trí, bước hiện tại");
    expect(list).toHaveTextContent("Đã xác nhận, sắp tới");
  });
});
