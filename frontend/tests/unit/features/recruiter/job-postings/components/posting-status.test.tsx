import { describe, expect, it } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { PostingStatus } from "@/features/recruiter/job-postings/components/posting-status";

describe("PostingStatus", () => {
  it("distinguishes an approved posting waiting for funding from a live vacancy", () => {
    const props = {
      status: "open" as const,
      questionBankReady: true,
      reviewSubmitted: true,
      interviewSlot: null,
    };
    renderWithIntl(<PostingStatus {...props} />);

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
    renderWithIntl(<PostingStatus {...props} />);

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
    renderWithIntl(<PostingStatus {...props} />);

    expect(screen.getByText("Intake closed")).toBeInTheDocument();
    expect(screen.queryByText("Live")).not.toBeInTheDocument();
  });
});

it("does not claim a posting is live before its funding state has loaded", () => {
  renderWithIntl(
    <PostingStatus status="open" questionBankReady reviewSubmitted />,
  );
  expect(screen.getByText("Approved")).toBeInTheDocument();
  expect(screen.queryByText("Live")).not.toBeInTheDocument();
});

describe("PostingStatus in Vietnamese", () => {
  const props = { questionBankReady: true, reviewSubmitted: true };

  it("labels each stage in Vietnamese", () => {
    const cases = [
      [
        { status: "open" as const, interviewSlot: null },
        "Đã duyệt · Cần nạp lượt phỏng vấn",
      ],
      [{ status: "open" as const, interviewSlot: 2 }, "Đang tuyển"],
      [{ status: "pending" as const }, "Đang chờ duyệt"],
      [
        { status: "intake_closed" as const, interviewSlot: 2 },
        "Đã ngừng nhận đơn",
      ],
      [{ status: "closed" as const }, "Đã đóng"],
      [{ status: "rejected" as const }, "Bị từ chối"],
    ] as const;
    for (const [extra, label] of cases) {
      const { unmount } = renderWithIntl(
        <PostingStatus {...props} {...extra} />,
        { locale: "vi" },
      );
      expect(screen.getByText(label)).toBeInTheDocument();
      unmount();
    }
  });

  it("tells a posting still being prepared from one waiting for review", () => {
    const { rerender } = renderWithIntl(
      <PostingStatus
        status="pending"
        questionBankReady={false}
        reviewSubmitted={false}
      />,
      { locale: "vi" },
    );
    expect(screen.getByText("Đang chuẩn bị")).toBeInTheDocument();
    rerender(
      <PostingStatus
        status="pending"
        questionBankReady
        reviewSubmitted={false}
      />,
    );
    expect(screen.getByText("Sẵn sàng gửi duyệt")).toBeInTheDocument();
  });
});
