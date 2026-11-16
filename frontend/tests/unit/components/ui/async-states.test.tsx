import { describe, expect, it, vi } from "vitest";
import { fireEvent, screen } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { ErrorState } from "@/components/ui/error-state";
import { AiLoader } from "@/components/ui/ai-loader";
import { LoadingMorph } from "@/components/ui/loading-morph";

describe("Shared asynchronous feedback", () => {
  it("prevents duplicate retries while recovery is running", () => {
    const retry = vi.fn();
    const { unmount } = renderWithIntl(
      <ErrorState
        variant="compact"
        description="Connection failed"
        isRetrying
        onRetry={retry}
      />,
    );
    expect(screen.getByRole("alert")).toHaveAttribute("aria-busy", "true");
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(retry).not.toHaveBeenCalled();
    unmount();
    renderWithIntl(
      <ErrorState
        variant="compact"
        description="Connection failed"
        onRetry={retry}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(retry).toHaveBeenCalledTimes(1);
  });

  it("announces the task and retains the report heading while waiting", () => {
    const { unmount } = renderWithIntl(
      <AiLoader
        stage="evaluating"
        title="Evaluating your interview"
        headingLevel={2}
      />,
    );
    expect(
      screen.getByRole("heading", {
        level: 2,
        name: "Evaluating your interview",
      }),
    ).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveAttribute("aria-live", "polite");
    unmount();
    renderWithIntl(
      <LoadingMorph variant="compact" label="Loading topic history" />,
    );
    expect(
      screen.getByRole("status", { name: "Loading topic history" }),
    ).toHaveTextContent("Loading topic history");
  });
});
