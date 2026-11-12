import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { fireEvent, screen } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { ActionSearchBar } from "@/components/ui/action-search-bar";

const options = [
  { value: "", label: "No rubric" },
  { value: "technical", label: "Đánh giá kỹ thuật" },
  { value: "archived", label: "Archived", disabled: true },
  { value: "communication", label: "Communication" },
];
function Picker({
  onChange = () => {},
}: {
  onChange?: (value: string) => void;
}) {
  const [value, setValue] = useState("technical");
  return (
    <ActionSearchBar
      options={options}
      value={value}
      onValueChange={(next) => {
        setValue(next);
        onChange(next);
      }}
      label="Rubric"
      placeholder="Choose rubric"
      searchPlaceholder="Search rubrics"
      emptyMessage="No matches"
    />
  );
}
describe("ActionSearchBar", () => {
  it("filters without accents, selects with the keyboard, and retains the selection on Escape", () => {
    const changed = vi.fn();
    renderWithIntl(<Picker onChange={changed} />);
    const input = screen.getByRole("combobox", { name: "Rubric" });
    expect(input).toHaveValue("Đánh giá kỹ thuật");
    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: "danh gia" } });
    expect(screen.getAllByRole("option")).toHaveLength(1);
    fireEvent.keyDown(input, { key: "Enter" });
    expect(changed).toHaveBeenCalledWith("technical");
    expect(input).toHaveAttribute("aria-expanded", "false");
    fireEvent.click(input);
    fireEvent.change(input, { target: { value: "nothing matches" } });
    expect(screen.getByRole("status")).toHaveTextContent("No matches");
    fireEvent.keyDown(input, { key: "Escape" });
    expect(input).toHaveValue("Đánh giá kỹ thuật");
    expect(changed).toHaveBeenCalledTimes(1);
  });
  it("skips disabled options and supports explicitly clearing the selection", () => {
    renderWithIntl(<Picker />);
    const input = screen.getByRole("combobox");
    fireEvent.focus(input);
    fireEvent.keyDown(input, { key: "ArrowDown" });
    expect(
      document.getElementById(
        input.getAttribute("aria-activedescendant") ?? "",
      ),
    ).toHaveTextContent("Communication");
    fireEvent.keyDown(input, { key: "Enter" });
    expect(input).toHaveValue("Communication");
    fireEvent.click(input);
    fireEvent.click(screen.getByRole("option", { name: "No rubric" }));
    expect(input).toHaveValue("No rubric");
    expect(input).toHaveAttribute("aria-expanded", "false");
  });
});
