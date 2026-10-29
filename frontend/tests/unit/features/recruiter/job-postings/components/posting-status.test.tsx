import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { PostingStatus } from "@/features/recruiter/job-postings/components/posting-status";

describe("PostingStatus", () => {
  it("distinguishes an approved posting waiting for funding from a live vacancy", () => {
    const props = {
      status: "open" as const,
      questionBankReady: true,
      reviewSubmitted: true,
      interviewSlot: null,
    };
    render(<PostingStatus {...props} />);

    expect(screen.getByText("Approved · Funding required")).toBeInTheDocument();
    expect(screen.queryByText("Live")).not.toBeInTheDocument();
  });

  it("marks an approved and funded posting as live", () => {
    const props = {
      status: "open" as const,
      questionBankReady: true,
      reviewSubmitted: true,
      interviewSlot: 2,
    };
    render(<PostingStatus {...props} />);

    expect(screen.getByText("Live")).toBeInTheDocument();
    expect(
      screen.queryByText("Approved · Funding required"),
    ).not.toBeInTheDocument();
  });

  it("keeps closed intake distinct from a live posting even when slots are funded", () => {
    const props = {
      status: "intake_closed" as const,
      questionBankReady: true,
      reviewSubmitted: true,
      interviewSlot: 2,
    };
    render(<PostingStatus {...props} />);

    expect(screen.getByText("Intake closed")).toBeInTheDocument();
    expect(screen.queryByText("Live")).not.toBeInTheDocument();
  });
});

it("does not claim a posting is live before its funding state has loaded", () => {
  render(<PostingStatus status="open" questionBankReady reviewSubmitted />);
  expect(screen.getByText("Approved")).toBeInTheDocument();
  expect(screen.queryByText("Live")).not.toBeInTheDocument();
});
