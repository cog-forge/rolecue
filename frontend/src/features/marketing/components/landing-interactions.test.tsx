import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { expect, it } from "vitest";
import { FaqSection } from "./faq-section";
import { PracticeSection } from "./practice-section";
import { RoleCueLanding } from "./rolecue-landing";

it("uses Radix Tabs keyboard behavior to switch the practice stage", async () => {
  render(<PracticeSection />);

  const roleTab = screen.getByRole("tab", { name: /read the role/i });
  roleTab.focus();
  fireEvent.keyDown(roleTab, { key: "ArrowRight" });

  const responseTab = screen.getByRole("tab", { name: /try the response/i });

  await waitFor(() =>
    expect(responseTab).toHaveAttribute("aria-selected", "true"),
  );
  const responsePanel = screen.getByRole("tabpanel");
  expect(responsePanel).toHaveAttribute("data-stage", "response");
  expect(responsePanel).toHaveAttribute(
    "aria-describedby",
    "practice-stage-response-description",
  );
  expect(
    screen.getByRole("heading", {
      name: "Give the answer room to take shape.",
    }),
  ).toBeVisible();
});

it("opens and closes an FAQ response with shadcn Accordion state", async () => {
  render(<FaqSection />);

  const question = screen.getByRole("button", {
    name: /is rolecue trying to script an interview/i,
  });
  fireEvent.click(question);

  expect(question).toHaveAttribute("aria-expanded", "true");
  await waitFor(() =>
    expect(
      screen.getByText(/centers reflection, examples, and clearer choices/i),
    ).toBeVisible(),
  );

  fireEvent.click(question);
  expect(question).toHaveAttribute("aria-expanded", "false");
});

it("targets the method section from the hero CTA", () => {
  render(<RoleCueLanding />);

  expect(screen.getByRole("link", { name: "See the method" })).toHaveAttribute(
    "href",
    "#method",
  );
});
