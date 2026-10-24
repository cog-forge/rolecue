import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { ApplicationDetail } from "@/features/recruiter/applications/components/application-detail";

// Deadlines show in the viewer's time zone; pin one (before the component is imported) so the
// rendered text is the same on every machine.
vi.hoisted(() => {
  process.env.TZ = "Asia/Ho_Chi_Minh";
});

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
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
vi.mock("@/lib/api/client", () => ({ apiClient: { get: mocks.get } }));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));

const postingId = "33333333-3333-4333-8333-333333333333";
const applicationId = "44444444-4444-4444-8444-000000000001";
const cvUrl = "https://files.example.com/cv/linh.pdf?signature=test";

// The detail response, as the Go API sends it (snake_case, no storage key, transcript or evaluation).
const detail = (
  over: Record<string, unknown> = {},
  posting: Record<string, unknown> = {},
) => ({
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
    ...posting,
  },
  cv: { url: cvUrl, expires_at: "2026-10-24T10:05:00Z" },
  ...over,
});
const ok = (data: ReturnType<typeof detail>) => ({
  data: { success: true, data },
});
function failure(status: number) {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: {
      success: false,
      error: { code: "INTERNAL_ERROR", message: "boom" },
    },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}
function mount() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  client.setQueryData(["auth", "me"], mocks.user);
  render(
    <QueryClientProvider client={client}>
      <ApplicationDetail postingId={postingId} applicationId={applicationId} />
    </QueryClientProvider>,
  );
  return client;
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
});

describe("ApplicationDetail", () => {
  it("shows a loading state until the Application arrives", async () => {
    mocks.get.mockResolvedValue(ok(detail()));
    mount();
    expect(screen.getByRole("status")).toHaveTextContent(
      /loading the application/i,
    );
    expect(
      await screen.findByRole("heading", { level: 1, name: "Linh Tran" }),
    ).toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledWith(
      `/job-postings/${postingId}/applications/${applicationId}`,
      { signal: expect.any(AbortSignal) },
    );
  });

  it("offers both CV decisions while a slot is free", async () => {
    mocks.get.mockResolvedValue(ok(detail()));
    mount();
    await screen.findByRole("heading", { level: 1, name: "Linh Tran" });
    expect(screen.getByRole("button", { name: "Approve CV" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Reject CV" })).toBeEnabled();
    expect(
      screen.queryByText("Approval is unavailable"),
    ).not.toBeInTheDocument();
  });

  it("explains that capacity is not funded, and keeps Reject", async () => {
    mocks.get.mockResolvedValue(
      ok(detail({}, { interview_slot: null, cv_approvals: 0 })),
    );
    mount();
    await screen.findByRole("heading", { level: 1, name: "Linh Tran" });
    expect(screen.getByRole("button", { name: "Approve CV" })).toBeDisabled();
    expect(
      screen.getByText(/Interview capacity is not funded/),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Reject CV" })).toBeEnabled();
  });

  it("explains that every slot is taken, and keeps Reject", async () => {
    mocks.get.mockResolvedValue(
      ok(detail({}, { interview_slot: 2, cv_approvals: 2 })),
    );
    mount();
    await screen.findByRole("heading", { level: 1, name: "Linh Tran" });
    expect(screen.getByRole("button", { name: "Approve CV" })).toBeDisabled();
    expect(
      screen.getByText(/All interview slots for this posting are taken/),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Reject CV" })).toBeEnabled();
  });

  it("offers no decision once intake is closed", async () => {
    mocks.get.mockResolvedValue(ok(detail({}, { status: "intake_closed" })));
    mount();
    await screen.findByRole("heading", { level: 1, name: "Linh Tran" });
    expect(screen.getByText("Intake is closed")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Approve CV" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Reject CV" }),
    ).not.toBeInTheDocument();
  });

  it("shows the interview deadline for an eligible Application, with no decision", async () => {
    mocks.get.mockResolvedValue(
      ok(
        detail({
          status: "interview_eligible",
          interview_eligible: true,
          interview_deadline: "2026-10-21T08:30:00Z",
        }),
      ),
    );
    mount();
    await screen.findByRole("heading", { level: 1, name: "Linh Tran" });
    expect(screen.getByText("Interview eligible")).toBeInTheDocument();
    expect(screen.getByText("Complete interview by")).toBeInTheDocument();
    expect(screen.getByText("Oct 21, 2026, 3:30 PM GMT+7")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Approve CV" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Reject CV" }),
    ).not.toBeInTheDocument();
  });

  it("shows a rejected Application with its stored reason and former deadline", async () => {
    mocks.get.mockResolvedValue(
      ok(
        detail({
          status: "rejected",
          interview_deadline: "2026-10-19T08:30:00Z",
          reject_reason: "Intake closed before this CV was screened.",
        }),
      ),
    );
    mount();
    await screen.findByRole("heading", { level: 1, name: "Linh Tran" });
    expect(screen.getByText("Rejected")).toBeInTheDocument();
    expect(screen.getByText("Interview deadline")).toBeInTheDocument();
    expect(
      screen.getByText("Intake closed before this CV was screened."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Reject CV" }),
    ).not.toBeInTheDocument();
  });

  it.each(["interviewed", "approved"])(
    "shows only the status for a %s Application",
    async (status) => {
      mocks.get.mockResolvedValue(ok(detail({ status })));
      mount();
      await screen.findByRole("heading", { level: 1, name: "Linh Tran" });
      expect(
        screen.queryByText(/deadline|Reject reason/),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: "Approve CV" }),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: "Reject CV" }),
      ).not.toBeInTheDocument();
    },
  );

  it("renders no transcript, recording or storage field", async () => {
    mocks.get.mockResolvedValue(ok(detail()));
    mount();
    await screen.findByRole("heading", { level: 1, name: "Linh Tran" });
    expect(
      screen.queryByText(/transcript|recording|storage/i),
    ).not.toBeInTheDocument();
  });

  it("shows not found, with a way back to the list", async () => {
    mocks.get.mockRejectedValue(failure(404));
    mount();
    expect(
      await screen.findByText("We couldn't find this application"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Back to received applications" }),
    ).toHaveAttribute("href", "/recruiter/applications");
    expect(
      screen.queryByRole("button", { name: "Try again" }),
    ).not.toBeInTheDocument();
  });

  it("shows access denied and re-checks the session", async () => {
    mocks.get.mockRejectedValue(failure(403));
    const client = mount();
    expect(
      await screen.findByText("You don't have access to this application"),
    ).toBeInTheDocument();
    expect(client.getQueryState(["auth", "me"])?.isInvalidated).toBe(true);
  });

  it("shows an error with a retry, and loads the Application on retry", async () => {
    mocks.get.mockRejectedValueOnce(failure(500));
    mount();
    expect(
      await screen.findByText("We couldn't load this application"),
    ).toBeInTheDocument();
    mocks.get.mockResolvedValue(ok(detail()));
    screen.getByRole("button", { name: "Try again" }).click();
    expect(
      await screen.findByRole("heading", { level: 1, name: "Linh Tran" }),
    ).toBeInTheDocument();
  });
});
