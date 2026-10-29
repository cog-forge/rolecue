import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import type { AppLocale } from "@/lib/i18n/config";
import { CandidateApplicationDetail } from "@/features/candidate/applications/components/application-detail";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  user: {
    id: "11111111-1111-4111-8111-111111111111",
    full_name: "Cand",
    email: "cand@example.com",
    role: "candidate" as const,
    email_verified: true,
    is_locked: false,
    image: null,
  },
}));
vi.mock("@/lib/api/client", () => ({ apiClient: { get: mocks.get } }));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));

const id = "44444444-4444-4444-8444-444444444444";
const hoursFromNow = (hours: number) =>
  new Date(Date.now() + hours * 3_600_000).toISOString();

const application = (over: Record<string, unknown> = {}) => ({
  data: {
    success: true,
    data: {
      id,
      job_posting_id: "33333333-3333-4333-8333-333333333333",
      job_posting_title: "Go Backend Engineer",
      status: "pending",
      reject_reason: null,
      interview_deadline: null,
      can_start_interview: false,
      created_at: "2026-10-20T08:00:00Z",
      ...over,
    },
  },
});

function failure(status: number) {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: { success: false, error: { code: "X", message: "server says" } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}

function renderDetail(applicationId = id, locale: AppLocale = "en") {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  return renderWithIntl(
    <QueryClientProvider client={client}>
      <CandidateApplicationDetail id={applicationId} />
    </QueryClientProvider>,
    { locale },
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
});

describe("CandidateApplicationDetail", () => {
  it("shows a pending application with no interview action", async () => {
    mocks.get.mockResolvedValue(application());
    renderDetail();
    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Go Backend Engineer",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText("Awaiting CV review")).toBeInTheDocument();
    expect(screen.getByText(/waiting for review/i)).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: /(start|resume) interview/i }),
    ).not.toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledWith(`/applications/${id}`, {
      signal: expect.any(AbortSignal),
    });
  });

  it("shows only the interview status and score, never a transcript", async () => {
    mocks.get.mockResolvedValue(
      application({
        status: "interviewed",
        interview: {
          id: "55555555-5555-4555-8555-555555555555",
          completed_at: "2026-10-24T09:00:00Z",
          status: "completed",
          score: 78,
        },
      }),
    );
    renderDetail();
    expect(await screen.findByText("Interview result")).toBeInTheDocument();
    expect(screen.getByText("78 / 100")).toBeInTheDocument();
    expect(screen.queryByText(/transcript|recording/i)).not.toBeInTheDocument();
  });

  it("shows the deadline, a countdown and a start link to the preflight while eligible", async () => {
    mocks.get.mockResolvedValue(
      application({
        status: "interview_eligible",
        interview_deadline: hoursFromNow(3),
        can_start_interview: true,
      }),
    );
    renderDetail();
    expect(await screen.findByText("Interview deadline")).toBeInTheDocument();
    expect(screen.getByText("Time left")).toBeInTheDocument();
    // Just under three hours remain, so the countdown reads two hours and most of a third.
    expect(screen.getByText(/^2 h \d+ min left$/)).toBeInTheDocument();

    // Navigation only: nothing starts until the microphone check.
    const start = screen.getByRole("link", { name: "Start interview" });
    expect(start).toHaveAttribute(
      "href",
      `/interviews/new/preflight?applicationId=${id}`,
    );
    expect(
      screen.getByText("Free for you. Complete it before the deadline."),
    ).toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledTimes(1);
  });

  it("shows the same deadline countdown and a localized time in Vietnamese", async () => {
    mocks.get.mockResolvedValue(
      application({
        status: "interview_eligible",
        interview_deadline: hoursFromNow(3),
        can_start_interview: true,
      }),
    );
    renderDetail(id, "vi");
    expect(await screen.findByText(/^Còn 2 giờ \d+ phút$/)).toBeInTheDocument();
  });

  it("offers to resume an interview that is in progress", async () => {
    mocks.get.mockResolvedValue(
      application({
        status: "interview_eligible",
        interview_deadline: hoursFromNow(3),
        can_start_interview: true,
        interview: {
          id: "55555555-5555-4555-8555-555555555555",
          completed_at: null,
          status: "in_progress",
          score: null,
        },
      }),
    );
    renderDetail();
    expect(
      await screen.findByRole("link", { name: "Resume interview" }),
    ).toHaveAttribute("href", `/interviews/new/preflight?applicationId=${id}`);
    expect(
      screen.getByText(/resuming uses no new interview slot/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Start interview" }),
    ).not.toBeInTheDocument();
  });

  it("shows a deadline passed note and no start action after the deadline", async () => {
    mocks.get.mockResolvedValue(
      application({
        status: "interview_eligible",
        interview_deadline: "2026-10-01T08:00:00Z",
        can_start_interview: false,
      }),
    );
    renderDetail();
    expect(
      await screen.findByText(/deadline passed\. the interview window/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: /(start|resume) interview/i }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Time left")).not.toBeInTheDocument();
  });

  it("shows a rejection with its reason and no start action", async () => {
    mocks.get.mockResolvedValue(
      application({
        status: "rejected",
        reject_reason: "No production Go experience.",
      }),
    );
    renderDetail();
    expect(await screen.findByText("Not selected")).toBeInTheDocument();
    expect(
      screen.getByText("Reason: No production Go experience."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: /(start|resume) interview/i }),
    ).not.toBeInTheDocument();
  });

  it("shows an approved application without an interview action", async () => {
    mocks.get.mockResolvedValue(application({ status: "approved" }));
    renderDetail();
    expect(
      await screen.findByText("The recruiter approved your application."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: /(start|resume) interview/i }),
    ).not.toBeInTheDocument();
  });

  it("never shows transcript, audio or video", async () => {
    mocks.get.mockResolvedValue(
      application({
        status: "interview_eligible",
        interview_deadline: hoursFromNow(2),
        can_start_interview: true,
      }),
    );
    const { container } = renderDetail();
    await screen.findByRole("heading", { level: 1 });
    expect(container).not.toHaveTextContent(
      /transcript|audio|video|recording/i,
    );
  });

  it("says when the application cannot be found", async () => {
    mocks.get.mockRejectedValue(failure(404));
    renderDetail();
    expect(
      await screen.findByRole("heading", {
        name: "We couldn't find this application",
      }),
    ).toBeInTheDocument();
  });

  it("rejects a malformed id without calling the API", () => {
    renderDetail("not-a-uuid");
    expect(
      screen.getByRole("heading", { name: "This link isn't valid" }),
    ).toBeInTheDocument();
    expect(mocks.get).not.toHaveBeenCalled();
  });
});
