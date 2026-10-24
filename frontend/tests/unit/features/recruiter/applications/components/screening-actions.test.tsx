import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { ScreeningActions } from "@/features/recruiter/applications/components/screening-actions";
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
const applicationId = "44444444-4444-4444-8444-000000000001";
const detailKey = [
  "recruiter-applications",
  mocks.user.id,
  "detail",
  postingId,
  applicationId,
];

const application = (
  over: Partial<ApplicationDetail> = {},
): ApplicationDetail => ({
  id: applicationId,
  job_posting_id: postingId,
  job_posting_title: "Backend Engineer",
  status: "pending",
  submitted_at: "2026-10-20T08:00:00Z",
  candidate: {
    id: "55555555-5555-4555-8555-000000000001",
    name: "Linh Tran",
    email: "linh@example.com",
    image: null,
  },
  interview_eligible: false,
  interview_deadline: null,
  reject_reason: null,
  updated_at: "2026-10-20T08:00:00Z",
  job_posting: {
    id: postingId,
    title: "Backend Engineer",
    status: "open",
    interview_slot: 5,
    cv_approvals: 1,
  },
  cv: null,
  ...over,
});
const screening = (over: Record<string, unknown> = {}) => ({
  data: {
    success: true,
    data: {
      id: applicationId,
      job_posting_id: postingId,
      status: "interview_eligible",
      interview_deadline: "2099-01-01T08:00:00Z",
      reject_reason: null,
      updated_at: "2026-10-24T09:00:00Z",
      job_posting_status: "open",
      ...over,
    },
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
  render(
    <QueryClientProvider client={client}>
      <ScreeningActions
        postingId={postingId}
        applicationId={applicationId}
        application={app}
      />
    </QueryClientProvider>,
  );
  return client;
}

const approvePath = `/job-postings/${postingId}/applications/${applicationId}/approve-cv`;
const rejectPath = `/job-postings/${postingId}/applications/${applicationId}/reject-cv`;

beforeEach(() => {
  vi.clearAllMocks();
  mocks.post.mockReset();
});

describe("ScreeningActions", () => {
  it("approves after confirmation, writes the new state into the detail and toasts", async () => {
    mocks.post.mockResolvedValue(screening());
    const client = mount();
    fireEvent.click(screen.getByRole("button", { name: "Approve CV" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Approve this CV?",
    });
    expect(dialog).toHaveTextContent(/counts toward this posting/);
    fireEvent.click(
      await screen.findByRole("button", { name: "Confirm approval" }),
    );

    await waitFor(() => expect(mocks.post).toHaveBeenCalledTimes(1));
    expect(mocks.post).toHaveBeenCalledWith(approvePath, {});
    await waitFor(() =>
      expect(mocks.toast).toHaveBeenCalledWith("CV approved"),
    );
    expect(client.getQueryData<ApplicationDetail>(detailKey)?.status).toBe(
      "interview_eligible",
    );
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
  });

  it("rejects without a reason when the field is empty", async () => {
    mocks.post.mockResolvedValue(
      screening({ status: "rejected", reject_reason: null }),
    );
    mount();
    fireEvent.click(screen.getByRole("button", { name: "Reject CV" }));
    fireEvent.click(
      within(
        await screen.findByRole("dialog", { name: "Reject this CV?" }),
      ).getByRole("button", { name: "Reject CV" }),
    );

    await waitFor(() => expect(mocks.post).toHaveBeenCalledTimes(1));
    expect(mocks.post).toHaveBeenCalledWith(rejectPath, {});
    await waitFor(() =>
      expect(mocks.toast).toHaveBeenCalledWith("CV rejected"),
    );
  });

  it("sends the trimmed reason", async () => {
    mocks.post.mockResolvedValue(
      screening({ status: "rejected", reject_reason: "Missing Go experience" }),
    );
    mount();
    fireEvent.click(screen.getByRole("button", { name: "Reject CV" }));
    fireEvent.change(await screen.findByLabelText(/Reason for the candidate/), {
      target: { value: "  Missing Go experience  " },
    });
    fireEvent.click(
      within(
        await screen.findByRole("dialog", { name: "Reject this CV?" }),
      ).getByRole("button", { name: "Reject CV" }),
    );

    await waitFor(() =>
      expect(mocks.post).toHaveBeenCalledWith(rejectPath, {
        reject_reason: "Missing Go experience",
      }),
    );
  });

  it("blocks a reason of 501 characters and allows 500", async () => {
    mocks.post.mockResolvedValue(screening({ status: "rejected" }));
    mount();
    fireEvent.click(screen.getByRole("button", { name: "Reject CV" }));
    const reason = await screen.findByLabelText(/Reason for the candidate/);

    fireEvent.change(reason, { target: { value: "a".repeat(501) } });
    fireEvent.click(
      within(
        await screen.findByRole("dialog", { name: "Reject this CV?" }),
      ).getByRole("button", { name: "Reject CV" }),
    );
    expect(
      await screen.findByText("Keep the reason to 500 characters or fewer."),
    ).toBeInTheDocument();
    expect(mocks.post).not.toHaveBeenCalled();

    fireEvent.change(reason, { target: { value: "a".repeat(500) } });
    fireEvent.click(
      within(
        await screen.findByRole("dialog", { name: "Reject this CV?" }),
      ).getByRole("button", { name: "Reject CV" }),
    );
    await waitFor(() => expect(mocks.post).toHaveBeenCalledTimes(1));
    expect(mocks.post).toHaveBeenCalledWith(rejectPath, {
      reject_reason: "a".repeat(500),
    });
  });

  it("shows the capacity message and reloads the detail when the slots are full", async () => {
    mocks.post.mockRejectedValue(failure(409, "INTERVIEW_CAPACITY_FULL"));
    const client = mount();
    fireEvent.click(screen.getByRole("button", { name: "Approve CV" }));
    fireEvent.click(
      await screen.findByRole("button", { name: "Confirm approval" }),
    );

    expect(
      await screen.findByText(
        "All interview slots for this posting are already taken.",
      ),
    ).toBeInTheDocument();
    expect(client.getQueryState(detailKey)?.isInvalidated).toBe(true);
    expect(mocks.toast).not.toHaveBeenCalled();
  });

  it("keeps the failure message visible once the reload shows the Application decided", async () => {
    mocks.post.mockRejectedValue(failure(409, "APPLICATION_NOT_PENDING"));
    const client = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });
    client.setQueryData(["auth", "me"], mocks.user);
    const tree = (app: ApplicationDetail) => (
      <QueryClientProvider client={client}>
        <ScreeningActions
          postingId={postingId}
          applicationId={applicationId}
          application={app}
        />
      </QueryClientProvider>
    );
    const view = render(tree(application()));
    fireEvent.click(screen.getByRole("button", { name: "Reject CV" }));
    fireEvent.click(
      within(
        await screen.findByRole("dialog", { name: "Reject this CV?" }),
      ).getByRole("button", { name: "Reject CV" }),
    );
    await screen.findByText(/already been decided/);

    view.rerender(tree(application({ status: "rejected" })));

    expect(screen.getByText(/already been decided/)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Reject CV" }),
    ).not.toBeInTheDocument();
  });

  it("reloads the detail when the Application was already decided", async () => {
    mocks.post.mockRejectedValue(failure(409, "APPLICATION_NOT_PENDING"));
    const client = mount();
    fireEvent.click(screen.getByRole("button", { name: "Reject CV" }));
    fireEvent.click(
      within(
        await screen.findByRole("dialog", { name: "Reject this CV?" }),
      ).getByRole("button", { name: "Reject CV" }),
    );

    expect(
      await screen.findByText(
        "This application has already been decided. Refresh to see its current status.",
      ),
    ).toBeInTheDocument();
    expect(client.getQueryState(detailKey)?.isInvalidated).toBe(true);
  });

  it("re-checks the session when access is denied", async () => {
    mocks.post.mockRejectedValue(failure(403, "FORBIDDEN"));
    const client = mount();
    fireEvent.click(screen.getByRole("button", { name: "Approve CV" }));
    fireEvent.click(
      await screen.findByRole("button", { name: "Confirm approval" }),
    );

    await waitFor(() =>
      expect(client.getQueryState(["auth", "me"])?.isInvalidated).toBe(true),
    );
  });
});
