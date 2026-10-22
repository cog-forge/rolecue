import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CopyButton } from "@/features/auth/components/copy-button";

function clipboard(writeText: (value: string) => Promise<void>) {
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText },
    configurable: true,
  });
}

describe("copy button", () => {
  afterEach(() => vi.useRealTimers());

  it("copies the value, swaps to the check and announces it", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    clipboard(writeText);
    render(<CopyButton value="JBSWY3DP" label="Copy setup key" />);
    fireEvent.click(screen.getByRole("button", { name: "Copy setup key" }));
    expect(await screen.findByRole("status")).toHaveTextContent(
      "Copied to clipboard.",
    );
    expect(writeText).toHaveBeenCalledWith("JBSWY3DP");
    expect(document.querySelector(".t-icon-swap")).toHaveAttribute(
      "data-state",
      "b",
    );
  });

  it("tells the user to copy manually when the clipboard is blocked", async () => {
    clipboard(vi.fn().mockRejectedValue(new Error("denied")));
    render(
      <CopyButton value="codes" label="Copy codes">
        Copy codes
      </CopyButton>,
    );
    fireEvent.click(screen.getByRole("button", { name: "Copy codes" }));
    await waitFor(() =>
      expect(screen.getByRole("status")).toHaveTextContent(/copy it manually/),
    );
    expect(document.querySelector(".t-icon-swap")).toHaveAttribute(
      "data-state",
      "a",
    );
  });
});
