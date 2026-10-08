import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { FaqSection } from "./faq-section";
import { HeroLottieSequence } from "./hero-lottie-sequence";
import { RoleCueLanding } from "./rolecue-landing";
import { WorkWheelSection } from "./work-wheel-section";

it("renders the new hero with Walk in ready headline and Start now CTA", () => {
  render(<RoleCueLanding />);

  // Single semantic h1
  const heading = screen.getByRole("heading", {
    level: 1,
    name: "Walk in ready.",
  });
  expect(heading).toBeInTheDocument();

  // CTA button pointing to canonical new interview route
  const cta = screen.getByRole("link", { name: /start now/i });
  expect(cta).toBeInTheDocument();
  expect(cta).toHaveAttribute("href", "/interviews/new/job-description");

  // Supporting copy and role-based factual pocket
  expect(
    screen.getByText("Practice the role before the room."),
  ).toBeInTheDocument();
  expect(screen.getByText("Real interview practice")).toBeInTheDocument();
  expect(
    screen.getByText(/Get feedback, build confidence/),
  ).toBeInTheDocument();
  expect(screen.getByText("ROLE-BASED")).toBeInTheDocument();
  expect(screen.getByText(/Practice from the actual/)).toBeInTheDocument();
});

it("renders the navbar and mobile navigation trigger", () => {
  render(<RoleCueLanding />);

  expect(
    screen.getByRole("navigation", { name: "Main navigation" }),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("button", { name: "Open navigation" }),
  ).toBeInTheDocument();
});

it("respects reduced-motion preference in the Lottie sequence", () => {
  // When matchMedia matches prefers-reduced-motion: reduce
  const originalMatchMedia = window.matchMedia;
  window.matchMedia = (query: string) => ({
    matches: query.includes("prefers-reduced-motion"),
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  });

  try {
    const { container } = render(<HeroLottieSequence />);
    const sequenceEl = container.querySelector(
      '[data-testid="hero-lottie-sequence"]',
    );
    expect(sequenceEl).toBeInTheDocument();
    // Reduced motion must stay on Interview.lottie (step "interview")
    expect(sequenceEl).toHaveAttribute("data-active-step", "interview");
  } finally {
    window.matchMedia = originalMatchMedia;
  }
});

it("moves through the RoleCue artwork", async () => {
  const width = vi
    .spyOn(HTMLElement.prototype, "clientWidth", "get")
    .mockReturnValue(1440);
  const height = vi
    .spyOn(HTMLElement.prototype, "clientHeight", "get")
    .mockReturnValue(684);
  try {
    render(<WorkWheelSection />);
    const choice = screen.getByRole("button", {
      name: "Show Set up your session",
    });
    fireEvent.click(choice);
    await waitFor(() => expect(choice).toHaveAttribute("aria-pressed", "true"));
    expect(
      screen.getByRole("listbox", { name: "The practice loop" }),
    ).toBeVisible();
  } finally {
    width.mockRestore();
    height.mockRestore();
  }
});

it("opens and closes an FAQ response with shadcn Accordion state", async () => {
  render(<FaqSection />);

  const question = screen.getByRole("button", {
    name: /can i start for free/i,
  });
  fireEvent.click(question);

  expect(question).toHaveAttribute("aria-expanded", "true");
  await waitFor(() =>
    expect(
      screen.getByText(
        /you can create an account and start practicing for free/i,
      ),
    ).toBeVisible(),
  );

  fireEvent.click(question);
  expect(question).toHaveAttribute("aria-expanded", "false");
});
