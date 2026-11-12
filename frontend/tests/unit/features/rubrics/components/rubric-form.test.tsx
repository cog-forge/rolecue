import { fireEvent, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "@tests/helpers/intl";
import { RubricForm } from "@/features/rubrics/components/rubric-form";
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));
const props = {
  rubric: null,
  metrics: [{ id: "11111111-1111-4111-8111-111111111111", name: "Accuracy" }],
  pending: false,
  error: null,
  conflict: false,
  onSave: vi.fn(),
  onClose: vi.fn(),
  onReload: vi.fn(),
  onReapply: vi.fn(),
};
describe("rubric form", () => {
  it("retains field errors and blocks an invalid save", async () => {
    renderWithIntl(<RubricForm {...props} />);
    fireEvent.click(screen.getByRole("button", { name: "Save rubric" }));
    await waitFor(() =>
      expect(
        screen.getByText(
          "Some fields need your attention. Your changes have not been saved.",
        ),
      ).toHaveTextContent("Some fields need your attention"),
    );
    expect(props.onSave).not.toHaveBeenCalled();
  });
  it("supports adding and removing criteria by keyboard accessible buttons", () => {
    renderWithIntl(<RubricForm {...props} />);
    expect(screen.getAllByLabelText("Criterion name")).toHaveLength(1);
    fireEvent.click(screen.getByRole("button", { name: "Add criterion" }));
    expect(screen.getAllByLabelText("Criterion name")).toHaveLength(2);
    fireEvent.click(screen.getByRole("button", { name: "Remove criterion 2" }));
    expect(screen.getAllByLabelText("Criterion name")).toHaveLength(1);
  });
  it("renders Vietnamese labels and saving feedback", () => {
    renderWithIntl(<RubricForm {...props} pending />, { locale: "vi" });
    expect(screen.getByLabelText("Tên barem")).toBeDisabled();
    expect(screen.getByRole("button", { name: "Đang lưu…" })).toBeDisabled();
  });
  it("holds the draft and blocks saving on conflict", () => {
    renderWithIntl(<RubricForm {...props} conflict error="Stale" />);
    expect(screen.getByRole("button", { name: "Save rubric" })).toBeDisabled();
    fireEvent.click(
      screen.getByRole("button", { name: "Keep edits with latest revision" }),
    );
    expect(props.onReapply).toHaveBeenCalledOnce();
  });
  it("uses the shared discard dialog and preserves input when cancelled", () => {
    const onClose = vi.fn();
    renderWithIntl(<RubricForm {...props} onClose={onClose} />);
    fireEvent.change(screen.getByLabelText("Rubric name", { exact: true }), {
      target: { value: "Draft" },
    });
    fireEvent.click(
      screen.getAllByRole("button", { name: "Back to library" })[0],
    );
    expect(screen.getByRole("dialog")).toHaveTextContent(
      "Discard your unsaved changes?",
    );
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.getByLabelText("Rubric name", { exact: true })).toHaveValue(
      "Draft",
    );
    expect(onClose).not.toHaveBeenCalled();
  });
});
