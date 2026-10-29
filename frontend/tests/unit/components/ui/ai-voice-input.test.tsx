import { render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
vi.mock("motion/react", () => ({ useReducedMotion: () => true }));
import { AIVoiceInput } from "@/components/ui/ai-voice-input";
it("reflects real input levels, stays flat when inactive, and supports reduced motion", () => {
  const view = render(<AIVoiceInput active={false} level={0.8} />);
  expect(screen.getByRole("meter")).toHaveAttribute("aria-valuenow", "0");
  view.rerender(<AIVoiceInput active level={0.65} />);
  expect(screen.getByRole("meter")).toHaveAttribute("aria-valuenow", "65");
  expect(view.container.querySelector("span")).toHaveStyle({
    transform: "scaleY(0.650)",
  });
  view.rerender(<AIVoiceInput active={false} level={0.65} />);
  expect(screen.getByRole("meter")).toHaveAttribute("aria-valuenow", "0");
  expect(view.container.querySelector("span")).toHaveStyle({
    transform: "scaleY(0.060)",
  });
});
