import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import {
  PostingProgress,
  type ProgressPosting,
} from "@/features/recruiter/job-postings/components/posting-progress";

const posting = (over: Partial<NonNullable<ProgressPosting>> = {}) => ({
  status: "pending" as const,
  review_submitted: false,
  core_question_count: 8,
  interview_slot: null,
  ...over,
});

function steps(value: ProgressPosting) {
  render(<PostingProgress posting={value} />);
  const list = screen.getByRole("list", { name: "Posting progress" });
  return within(list)
    .getAllByRole("listitem")
    .map((item) => ({
      text: item.textContent ?? "",
      current: item.getAttribute("aria-current") === "step",
    }));
}

describe("PostingProgress", () => {
  it("runs from requirements to screening", () => {
    const labels = [
      "Requirements",
      "Questions",
      "Evaluation",
      "Admin review",
      "Fund slots",
      "Screening",
    ];
    const all = steps(posting());
    expect(all).toHaveLength(labels.length);
    labels.forEach((label, index) =>
      expect(all[index].text.startsWith(label)).toBe(true),
    );
  });

  it("marks evaluation as the optional step to do before submitting", () => {
    const all = steps(posting());
    const evaluation = all[2];
    expect(evaluation.current).toBe(true);
    expect(evaluation.text).toContain("Optional: defaults apply");
  });

  it("waits on the administrator once submitted", () => {
    const all = steps(posting({ review_submitted: true }));
    expect(all[3]).toMatchObject({ current: true });
    expect(all[3].text).toContain("Waiting for an administrator");
    expect(all[5].text).toContain("After approval and funding");
  });

  it("asks for funding once approved, and screening starts after it", () => {
    const approved = steps(posting({ status: "open" }));
    expect(approved[4]).toMatchObject({ current: true });
    expect(approved[4].text).toContain("Fund slots to publish");
    expect(approved[5].text).toContain("After funding");
    expect(approved[5].current).toBe(false);
  });

  it("screens while the posting takes applications", () => {
    const live = steps(posting({ status: "open", interview_slot: 5 }));
    expect(live[4].text).toContain("Slots funded");
    expect(live[5]).toMatchObject({ current: true });
    expect(live[5].text).toContain("Taking applications");
  });

  it("keeps screening while closed intake finishes its interviews", () => {
    const all = steps(posting({ status: "intake_closed", interview_slot: 5 }));
    expect(all[5]).toMatchObject({ current: true });
    expect(all[5].text).toContain("Intake closed, finishing interviews");
  });

  it("finishes screening when the posting closes, funded or not", () => {
    const funded = steps(posting({ status: "closed", interview_slot: 5 }));
    expect(funded[5].text).toContain("Closed");
    expect(funded[5].current).toBe(false);
    document.body.innerHTML = "";
    const unfunded = steps(posting({ status: "closed", interview_slot: null }));
    expect(unfunded[4].text).toContain("Never funded");
  });

  it("flags the admin step when changes were requested", () => {
    const all = steps(posting({ status: "rejected" }));
    expect(all[3].text).toContain("Changes requested");
    expect(all[5].text).toContain("After approval and funding");
  });
});
