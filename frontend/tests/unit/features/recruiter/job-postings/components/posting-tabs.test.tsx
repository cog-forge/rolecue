import { describe, expect, it } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { PostingTabs } from "@/features/recruiter/job-postings/components/posting-tabs";

const postingId = "33333333-3333-4333-8333-333333333333";

describe("PostingTabs", () => {
  it("links each section and marks the current one", () => {
    renderWithIntl(
      <PostingTabs postingId={postingId} active="questions" status="open" />,
    );

    expect(
      screen.getByRole("navigation", { name: "Posting sections" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Questions" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(screen.getByRole("link", { name: "Overview" })).toHaveAttribute(
      "href",
      `/recruiter/job-postings/${postingId}`,
    );
  });

  it("explains why Applications is off until the posting is approved", () => {
    renderWithIntl(
      <PostingTabs postingId={postingId} active="overview" status="pending" />,
    );

    const disabled = screen.getByRole("link", { name: /Applications/ });
    expect(disabled).toHaveAttribute("aria-disabled", "true");
    expect(disabled).toHaveAccessibleName(
      /\(available once an administrator approves the posting and you fund interview slots\)$/,
    );
    expect(disabled).toHaveAttribute(
      "title",
      "Available once an administrator approves the posting and you fund interview slots",
    );
  });

  it("is labelled in Vietnamese", () => {
    renderWithIntl(
      <PostingTabs postingId={postingId} active="overview" status="pending" />,
      { locale: "vi" },
    );

    expect(
      screen.getByRole("navigation", { name: "Các phần của tin tuyển dụng" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Tổng quan" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(screen.getByRole("link", { name: /Đơn ứng tuyển/ })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
  });
});
