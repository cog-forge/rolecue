import { describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { RoleReviewForm } from "@/features/role-profile/components/role-review-form";
import type { RoleProfile } from "@/features/role-profile/api/contract";

const profile = (over: Partial<RoleProfile> = {}): RoleProfile => ({
  id: "22222222-2222-4222-8222-222222222222",
  title: "Backend Engineer",
  rawText: "JD",
  hasOriginalDocument: false,
  seniorityLevel: "senior",
  requirements: [
    {
      name: "Go concurrency",
      target: "Build concurrent services",
      kind: "skill",
      category: "programming_language",
      type: "required",
      evidenceText: "We use Go every day.",
    },
  ],
  refinementNote: "Focus on caching",
  status: "review",
  createdAt: "2026-10-10T00:00:00Z",
  ...over,
});

function renderForm(p: RoleProfile, locale: "en" | "vi" = "vi") {
  const onSave = vi.fn().mockResolvedValue(p);
  const onConfirm = vi.fn().mockResolvedValue(p);
  renderWithIntl(
    <RoleReviewForm
      profile={p}
      busy={false}
      onSave={onSave}
      onConfirm={onConfirm}
    />,
    { locale },
  );
  return { onSave, onConfirm };
}

describe("RoleReviewForm in Vietnamese", () => {
  it("translates labels and the requirement classification, not AI content", () => {
    renderForm(profile());
    expect(screen.getByLabelText("Tên vị trí")).toHaveValue("Backend Engineer");
    expect(screen.getByLabelText("Cấp bậc")).toHaveTextContent("Senior");
    expect(
      screen.getByRole("heading", { name: /Yêu cầu kỹ thuật/ }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Yêu cầu 01")).toHaveValue("Go concurrency");
    // The stored legacy target is shown as written; the classification is a chrome label.
    expect(screen.getByText("Build concurrent services")).toBeInTheDocument();
    expect(screen.getByText("Ngôn ngữ · Bắt buộc")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Chi tiết của yêu cầu 1" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Xóa yêu cầu 1" }),
    ).toBeInTheDocument();
    expect(
      screen.getByLabelText("Biến buổi luyện tập thành của riêng bạn."),
    ).toHaveValue("Focus on caching");
    expect(screen.getByText("16 / 2.000")).toBeInTheDocument();
    expect(
      screen.getByText("Bản xem lại mới nhất của bạn đã được lưu."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Lưu bản xem lại" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Xác nhận vị trí" }),
    ).toBeInTheDocument();
  });

  it("shows the evidence quote unchanged under the details", () => {
    renderForm(profile());
    fireEvent.click(
      screen.getByRole("button", { name: "Chi tiết của yêu cầu 1" }),
    );
    expect(screen.getByText(/We use Go every day\./)).toBeInTheDocument();
    expect(screen.getByText("Từ JD")).toBeInTheDocument();
    expect(screen.getByText("Danh mục kỹ năng")).toBeInTheDocument();
    expect(screen.getByText("Mức ưu tiên")).toBeInTheDocument();
  });

  it("validates in Vietnamese through the schema factory", async () => {
    const { onSave } = renderForm(profile());
    fireEvent.change(screen.getByLabelText("Tên vị trí"), {
      target: { value: "" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Lưu bản xem lại" }));
    expect(
      await screen.findByText("Hãy nhập từ 1 đến 200 ký tự."),
    ).toBeInTheDocument();
    expect(onSave).not.toHaveBeenCalled();
  });

  it("asks for a seniority before confirming, in Vietnamese", async () => {
    const { onConfirm } = renderForm(profile({ seniorityLevel: null }));
    fireEvent.click(screen.getByRole("button", { name: "Xác nhận vị trí" }));
    expect(
      await screen.findByText("Hãy chọn cấp bậc trước khi xác nhận."),
    ).toBeInTheDocument();
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it("requires a skill category in the active language", async () => {
    renderForm(profile({ requirements: [] }));
    fireEvent.click(screen.getByRole("button", { name: "Lưu bản xem lại" }));
    await waitFor(() =>
      expect(
        screen.getByText("Hãy giữ lại ít nhất một yêu cầu kỹ thuật."),
      ).toBeInTheDocument(),
    );
  });

  it("keeps the English wording in English", () => {
    renderForm(profile(), "en");
    expect(screen.getByLabelText("Role title")).toBeInTheDocument();
    expect(screen.getByText("Language · Required")).toBeInTheDocument();
    expect(screen.getByText("16 / 2,000")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Confirm role" }),
    ).toBeInTheDocument();
  });
});
