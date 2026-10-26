import { render, screen } from "@testing-library/react";
import { expect, it } from "vitest";
import { RoutePlaceholder } from "@/components/feedback/route-placeholder";
it("exposes an accessible page heading and description", () => {
  render(
    <RoutePlaceholder
      title="Interview history"
      description="Review previous sessions."
    />,
  );
  expect(
    screen.getByRole("heading", { level: 1, name: "Interview history" }),
  ).toBeInTheDocument();
  expect(screen.getByText("Review previous sessions.")).toBeVisible();
});
