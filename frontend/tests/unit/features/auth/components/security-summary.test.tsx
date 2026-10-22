import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { SecuritySummary } from "@/features/auth/components/security-summary";

describe("security summary", () => {
  it("answers what protects the account and links each answer to its section", () => {
    render(
      <SecuritySummary
        emailVerified
        hasPassword
        twoFactorEnabled={false}
        sessionCount={3}
      />,
    );
    const summary = screen.getByRole("navigation", {
      name: "Security summary",
    });
    expect(summary).toHaveTextContent("Two-factorOffRecommended");
    expect(summary).toHaveTextContent(
      "Signed in3 sessionsThis browser and 2 more",
    );
    expect(screen.getByRole("link", { name: /Two-factor/ })).toHaveAttribute(
      "href",
      "#two-factor-settings",
    );
  });

  it("prints a dash, not a guess, while counts are unknown", () => {
    render(<SecuritySummary emailVerified={false} twoFactorEnabled />);
    expect(screen.getByRole("link", { name: /Password/ })).toHaveTextContent(
      "Password—",
    );
    expect(screen.getByRole("link", { name: /^Email/ })).toHaveTextContent(
      "Not verified",
    );
    expect(screen.getByRole("link", { name: /Two-factor/ })).toHaveTextContent(
      "On",
    );
  });
});
