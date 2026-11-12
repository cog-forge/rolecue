import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithIntl } from "@tests/helpers/intl";
import { MetricCoverage } from "@/components/feedback/metric-coverage";
import { coverageSchema, type Coverage } from "@/lib/api/coverage-schema";

const rubric = {
  id: "11111111-1111-4111-8111-111111111111",
  name: "Explain a mechanism",
};
const failing: Coverage = coverageSchema.parse({
  passes: false,
  assessed: true,
  covered: 3,
  total: 5,
  question_limit: 8,
  missing_metrics: [
    {
      id: "22222222-2222-4222-8222-222222222222",
      name: "Depth of Understanding",
      suggested_rubrics: [rubric],
    },
    {
      id: "33333333-3333-4333-8333-333333333333",
      name: "Problem-Solving & Approach",
      suggested_rubrics: [],
    },
  ],
});
const passing: Coverage = {
  ...failing,
  passes: true,
  covered: 5,
  missing_metrics: [],
};

describe("MetricCoverage", () => {
  it("shows a meter with one segment per metric, the missing metrics as chips and one short hint", () => {
    renderWithIntl(<MetricCoverage coverage={failing} audience="recruiter" />);

    const meter = screen.getByRole("meter", { name: "Metric coverage" });
    expect(meter).toHaveAttribute("aria-valuenow", "3");
    expect(meter).toHaveAttribute("aria-valuemax", "5");
    expect(meter).toHaveAttribute("aria-valuetext", "3 of 5 metrics covered");
    expect(meter.children).toHaveLength(5);
    expect(screen.getByText("3/5")).toBeInTheDocument();
    const panel = screen.getByRole("status");
    expect(within(panel).getByText("Not measured:")).toBeInTheDocument();
    expect(
      within(panel).getByText("Depth of Understanding"),
    ).toBeInTheDocument();
    expect(
      within(panel).getByText("Problem-Solving & Approach"),
    ).toBeInTheDocument();
    expect(
      within(panel).getByText(
        "Assign a rubric that covers them, for example Explain a mechanism.",
      ),
    ).toBeInTheDocument();
  });

  it("changes the meter colour with how much is covered", () => {
    const { rerender } = renderWithIntl(
      <MetricCoverage
        coverage={{ ...failing, covered: 1 }}
        audience="recruiter"
      />,
    );
    expect(screen.getByRole("meter")).toHaveAttribute("data-tone", "poor");
    rerender(
      <MetricCoverage
        coverage={{ ...failing, covered: 4 }}
        audience="recruiter"
      />,
    );
    expect(screen.getByRole("meter")).toHaveAttribute("data-tone", "partial");
    rerender(<MetricCoverage coverage={passing} audience="recruiter" />);
    expect(screen.getByRole("meter")).toHaveAttribute("data-tone", "ok");
  });

  it("tells the Admin the Recruiter has to fix the bank, without rubric hints", () => {
    renderWithIntl(<MetricCoverage coverage={failing} audience="admin" />);

    expect(
      screen.getByText(
        "The Recruiter must fix the question bank before approval.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/for example/)).not.toBeInTheDocument();
  });

  it("explains a bank whose overlap is too small for the interview length", () => {
    const exceeding = coverageSchema.parse({
      ...failing,
      covered: 4,
      question_limit: 2,
      min_questions: 3,
      exceeds_limit: true,
      missing_metrics: [],
    });
    renderWithIntl(
      <MetricCoverage coverage={exceeding} audience="recruiter" />,
    );

    expect(
      screen.getByText(
        "An interview asks 2 questions; this bank needs 3 to cover every metric.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Not measured:")).not.toBeInTheDocument();
  });

  it("confirms with a full green meter once every metric is covered", () => {
    renderWithIntl(<MetricCoverage coverage={passing} audience="recruiter" />);

    const status = screen.getByRole("status");
    expect(status).toHaveTextContent("All metrics covered");
    expect(status).toHaveTextContent("5/5");
    expect(status).toHaveAttribute("data-coverage", "ok");
    expect(screen.getByRole("meter")).toHaveAttribute("data-tone", "ok");
    expect(screen.queryByRole("heading")).not.toBeInTheDocument();
  });

  it("shows nothing without coverage or when there was nothing to assess", () => {
    const { container, rerender } = renderWithIntl(
      <MetricCoverage coverage={null} audience="recruiter" />,
    );
    expect(container).toBeEmptyDOMElement();
    rerender(
      <MetricCoverage
        coverage={{ ...failing, assessed: false }}
        audience="recruiter"
      />,
    );
    expect(container).toBeEmptyDOMElement();
    rerender(<MetricCoverage coverage={undefined} audience="admin" />);
    expect(container).toBeEmptyDOMElement();
  });

  it("is written in Vietnamese for a Vietnamese user", () => {
    renderWithIntl(<MetricCoverage coverage={failing} audience="recruiter" />, {
      locale: "vi",
    });

    expect(
      screen.getByRole("meter", { name: "Độ phủ tiêu chí" }),
    ).toHaveAttribute("aria-valuetext", "đã phủ 3/5 tiêu chí");
    expect(screen.getByText("Chưa có câu đo:")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Hãy gán barem phủ tiêu chí này, ví dụ Explain a mechanism.",
      ),
    ).toBeInTheDocument();
  });
});
