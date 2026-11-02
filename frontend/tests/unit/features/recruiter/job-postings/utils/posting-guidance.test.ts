import { describe, expect, it } from "vitest";
import { createTranslator } from "next-intl";
import { formattersFor, messagesFor } from "@tests/helpers/intl";
import type { AppLocale } from "@/lib/i18n/config";
import type { CapacityState } from "@/features/recruiter/capacity/schemas/capacity-schema";
import type { JobPosting } from "@/features/recruiter/job-postings/schemas/job-posting-schema";
import {
  nextStep,
  primaryAction,
  statusLine,
} from "@/features/recruiter/job-postings/utils/posting-guidance";

const guidanceFor = (locale: AppLocale = "en") =>
  createTranslator({
    locale,
    messages: messagesFor(locale),
    namespace: "recruiter.postings.guidance",
  });

const postingId = "33333333-3333-4333-8333-333333333333";
const posting = (over: Partial<JobPosting> = {}): JobPosting => ({
  id: postingId,
  title: "Backend Engineer",
  status: "pending",
  reject_reason: null,
  raw_text: "We build Go services.",
  has_document: false,
  seniority_level: "senior",
  requirements: [],
  refinement_note: "",
  location: null,
  employment_type: null,
  salary_min: null,
  salary_max: null,
  interview_duration: null,
  interview_slot: null,
  required_avatar: null,
  required_voice: null,
  core_question_count: 0,
  review_submitted: false,
  created_at: "2026-10-12T08:00:00Z",
  ...over,
});
const capacity = (over: Partial<CapacityState> = {}): CapacityState => ({
  interview_slot: 5,
  funded: true,
  price_per_slot: 100,
  cv_approvals: 2,
  consumed: 0,
  available_to_approve: 3,
  refundable: 0,
  posting_status: "open",
  refunded: null,
  ...over,
});

describe("primaryAction", () => {
  it("points at the next step in English", () => {
    const t = guidanceFor();
    expect(primaryAction(posting({ core_question_count: 4 }), t)).toEqual({
      label: "Review & submit",
      href: `/recruiter/job-postings/${postingId}/questions`,
    });
    expect(primaryAction(posting(), t)).toEqual({
      label: "Review requirements",
      href: "#posting-requirements",
    });
    expect(
      primaryAction(posting({ status: "open", interview_slot: null }), t)
        ?.label,
    ).toBe("Fund & publish");
    expect(primaryAction(posting({ status: "closed" }), t)).toBeNull();
  });

  it("labels the same links in Vietnamese", () => {
    const t = guidanceFor("vi");
    expect(
      primaryAction(posting({ status: "rejected", core_question_count: 2 }), t)
        ?.label,
    ).toBe("Chỉnh sửa và gửi lại");
    expect(
      primaryAction(posting({ status: "open", interview_slot: 3 }), t)?.label,
    ).toBe("Xem đơn ứng tuyển");
  });
});

describe("statusLine", () => {
  it("states slot progress and the refund with locale-formatted coins", () => {
    const open = posting({ status: "open", interview_slot: 5 });
    expect(statusLine(open, formattersFor(), guidanceFor(), capacity())).toBe(
      "2 of 5 interview slots approved",
    );
    expect(
      statusLine(open, formattersFor("vi"), guidanceFor("vi"), capacity()),
    ).toBe("Đã duyệt 2/5 lượt phỏng vấn");

    const closed = posting({ status: "closed", interview_slot: 5 });
    const refunded = capacity({ posting_status: "closed", refunded: 1500 });
    expect(statusLine(closed, formattersFor(), guidanceFor(), refunded)).toBe(
      "Closed: 1,500 coins from unused slots returned to your wallet",
    );
    expect(
      statusLine(closed, formattersFor("vi"), guidanceFor("vi"), refunded),
    ).toBe(
      "Đã đóng: 1.500 xu từ các lượt chưa dùng đã được hoàn vào ví của bạn",
    );
  });

  it("stays out of the way when the figures are unknown", () => {
    const open = posting({ status: "open", interview_slot: 5 });
    expect(statusLine(open, formattersFor(), guidanceFor())).toBeNull();
    expect(
      statusLine(posting({ status: "closed" }), formattersFor(), guidanceFor()),
    ).toBeNull();
  });
});

describe("nextStep", () => {
  it("describes what happens next for each stage", () => {
    const en = guidanceFor();
    const vi = guidanceFor("vi");
    const rejected = posting({ status: "rejected" });
    expect(nextStep(rejected, en)).toBe(
      "Make the requested changes, then resubmit the posting for approval.",
    );
    expect(nextStep(rejected, vi)).toBe(
      "Hãy chỉnh sửa theo yêu cầu, rồi gửi lại tin tuyển dụng để được duyệt.",
    );
    expect(
      nextStep(posting({ status: "open", interview_slot: null }), en),
    ).toBe(
      "Approved, so the posting can no longer be edited. Fund interview slots to open intake to candidates.",
    );
  });
});
