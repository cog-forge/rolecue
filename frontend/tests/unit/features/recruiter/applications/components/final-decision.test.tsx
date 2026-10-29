import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { FinalDecision } from "@/features/recruiter/applications/components/final-decision";
import type { ApplicationDetail } from "@/features/recruiter/applications/schemas/application-detail-schema";

const mocks = vi.hoisted(() => ({
  post: vi.fn(),
  toast: vi.fn(),
  user: {
    id: "11111111-1111-4111-8111-111111111111",
    full_name: "Rec",
    email: "rec@example.com",
    role: "recruiter" as const,
    email_verified: true,
    is_locked: false,
    image: null,
  },
}));
vi.mock("@/lib/api/client", () => ({ apiClient: { post: mocks.post } }));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));
vi.mock("goey-toast", () => ({ gooeyToast: { success: mocks.toast } }));

const postingId = "33333333-3333-4333-8333-333333333333";
const applicationId = "44444444-4444-4444-8444-444444444444";
const detailKey = [
  "recruiter-applications",
  mocks.user.id,
  "detail",
  postingId,
  applicationId,
];
const decisionPath = `/job-postings/${postingId}/applications/${applicationId}/decision`;

const application = (
  over: Partial<ApplicationDetail> = {},
): ApplicationDetail => ({
  id: applicationId,
  job_posting_id: postingId,
  job_posting_title: "Backend Engineer",
  status: "interviewed",
  submitted_at: "2026-10-20T08:00:00Z",
  candidate: {
    id: "55555555-5555-4555-8555-000000000001",
    name: "Linh Tran",
    email: "linh@example.com",
    image: null,
  },
  interview_eligible: false,
  interview_deadline: "2026-10-21T08:00:00Z",
  reject_reason: null,
  updated_at: "2026-10-22T09:00:00Z",
  job_posting: {
    id: postingId,
    title: "Backend Engineer",
    status: "open",
    interview_slot: 5,
    cv_approvals: 2,
  },
  cv: null,
  final_review: { opens_at: "2099-01-01T08:00:00Z", can_decide: false },
  ...over,
});

const decided = (
  status: ApplicationDetail["status"],
  reason: string | null = null,
) => ({
  data: {
    success: true,
    data: { ...application({ status }), reject_reason: reason },
  },
});

function failure(status: number, code: string, message = "boom") {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: { success: false, error: { code, message } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}

function mount(app: ApplicationDetail = application()) {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity },
      mutations: { retry: false },
    },
  });
  client.setQueryData(["auth", "me"], mocks.user);
  client.setQueryData(detailKey, app);
  renderWithIntl(
    <QueryClientProvider client={client}>
      <FinalDecision
        postingId={postingId}
        applicationId={applicationId}
        application={app}
      />
    </QueryClientProvider>,
  );
  return client;
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.post.mockReset();
});

describe("FinalDecision", () => {
  it("disables both decisions and shows when they open", () => {
    mount();
    expect(
      screen.getByRole("button", { name: "Approve application" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Reject application" }),
    ).toBeDisabled();
    expect(
      screen.getByText("Final decision is not open yet"),
    ).toBeInTheDocument();
    expect(screen.getByText(/Decisions open/)).toBeInTheDocument();
  });

  it("enables both decisions once the server allows them", () => {
    mount(
      application({
        final_review: { opens_at: "2026-10-21T08:00:00Z", can_decide: true },
      }),
    );
    expect(
      screen.getByRole("button", { name: "Approve application" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: "Reject application" }),
    ).toBeEnabled();
    expect(
      screen.queryByText("Final decision is not open yet"),
    ).not.toBeInTheDocument();
  });

  it("keeps both decisions disabled when the review gate is missing", () => {
    mount(application({ final_review: undefined }));
    expect(
      screen.getByRole("button", { name: "Approve application" }),
    ).toBeDisabled();
    expect(
      screen.getByText("Decisions are not open for this application yet."),
    ).toBeInTheDocument();
  });

  it("approves after confirmation and reloads the detail once the server confirms", async () => {
    mocks.post.mockResolvedValue(decided("approved"));
    const client = mount(
      application({
        final_review: { opens_at: "2026-10-21T08:00:00Z", can_decide: true },
      }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Approve application" }),
    );
    const dialog = await screen.findByRole("dialog", {
      name: "Approve this application?",
    });
    expect(dialog).toHaveTextContent("Linh Tran");
    fireEvent.click(
      await screen.findByRole("button", { name: "Confirm approval" }),
    );

    await waitFor(() =>
      expect(mocks.toast).toHaveBeenCalledWith("Application approved"),
    );
    expect(mocks.post).toHaveBeenCalledWith(decisionPath, {
      decision: "approve",
    });
    expect(client.getQueryState(detailKey)?.isInvalidated).toBe(true);
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
  });

  it("sends the trimmed rejection reason", async () => {
    mocks.post.mockResolvedValue(decided("rejected", "Weak testing"));
    mount(
      application({
        final_review: { opens_at: "2026-10-21T08:00:00Z", can_decide: true },
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Reject application" }));
    fireEvent.change(await screen.findByLabelText(/Reason for the candidate/), {
      target: { value: "  Weak testing  " },
    });
    fireEvent.click(
      within(
        await screen.findByRole("dialog", { name: "Reject this application?" }),
      ).getByRole("button", { name: "Reject application" }),
    );

    await waitFor(() =>
      expect(mocks.post).toHaveBeenCalledWith(decisionPath, {
        decision: "reject",
        reason: "Weak testing",
      }),
    );
    await waitFor(() =>
      expect(mocks.toast).toHaveBeenCalledWith("Application rejected"),
    );
  });

  it("rejects without a reason field when the reason is empty", async () => {
    mocks.post.mockResolvedValue(decided("rejected"));
    mount(
      application({
        final_review: { opens_at: "2026-10-21T08:00:00Z", can_decide: true },
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Reject application" }));
    fireEvent.click(
      within(
        await screen.findByRole("dialog", { name: "Reject this application?" }),
      ).getByRole("button", { name: "Reject application" }),
    );

    await waitFor(() =>
      expect(mocks.post).toHaveBeenCalledWith(decisionPath, {
        decision: "reject",
      }),
    );
  });

  it("explains a locked review and reloads the detail", async () => {
    mocks.post.mockRejectedValue(failure(409, "FINAL_REVIEW_LOCKED"));
    const client = mount(
      application({
        final_review: { opens_at: "2026-10-21T08:00:00Z", can_decide: true },
      }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Approve application" }),
    );
    fireEvent.click(
      await screen.findByRole("button", { name: "Confirm approval" }),
    );

    expect(
      await screen.findByText("Final decisions are not open yet."),
    ).toBeInTheDocument();
    expect(client.getQueryState(detailKey)?.isInvalidated).toBe(true);
    expect(mocks.toast).not.toHaveBeenCalled();
  });

  it("re-checks the session when access is denied", async () => {
    mocks.post.mockRejectedValue(failure(403, "FORBIDDEN"));
    const client = mount(
      application({
        final_review: { opens_at: "2026-10-21T08:00:00Z", can_decide: true },
      }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Approve application" }),
    );
    fireEvent.click(
      await screen.findByRole("button", { name: "Confirm approval" }),
    );

    await waitFor(() =>
      expect(client.getQueryState(["auth", "me"])?.isInvalidated).toBe(true),
    );
  });
});
