import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { JobPostingDetail } from "@/features/recruiter/job-postings/components/job-posting-detail";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  patch: vi.fn(),
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
vi.mock("@/lib/api/client", () => ({
  apiClient: { get: mocks.get, post: mocks.post, patch: mocks.patch },
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));
vi.mock("goey-toast", () => ({ gooeyToast: { success: mocks.toast } }));

const postingId = "33333333-3333-4333-8333-333333333333";
const avatarId = "44444444-4444-4444-8444-444444444444";
const voiceId = "55555555-5555-4555-8555-555555555555";
const oldVoiceId = "66666666-6666-4666-8666-666666666666";
const envelope = <T,>(data: T) => ({ data: { success: true, data } });
const pageOf = <T,>(items: T[]) =>
  envelope({
    items,
    pagination: { limit: 100, offset: 0, total: items.length },
  });
const base = {
  id: postingId,
  title: "Backend Engineer",
  status: "pending",
  reject_reason: null as string | null,
  raw_text: "We build Go services on PostgreSQL for payments. ".repeat(3),
  seniority_level: "senior",
  requirements: [
    { name: "Go", target: "programming_language", type: "required" },
  ],
  location: "Hanoi",
  employment_type: null,
  salary_min: null,
  salary_max: null,
  interview_duration: 45,
  interview_slot: null,
  required_avatar: { id: avatarId, name: "Linh" },
  required_voice: { id: voiceId, name: "Ava" },
  core_question_count: 0,
  created_at: "2026-10-12T08:00:00Z",
};
type Posting = typeof base;
function failure(status: number, code: string, message: string) {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: { success: false, error: { code, message } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}
function serve(posting: Posting | Error) {
  mocks.get.mockImplementation((path: string) => {
    if (path === "/voices")
      return Promise.resolve(pageOf([{ id: voiceId, name: "Ava" }]));
    if (path === "/inventory/avatars")
      return Promise.resolve(
        pageOf([
          {
            id: avatarId,
            name: "Linh",
            personality: "Calm",
            created_at: "2026-10-01T00:00:00Z",
            updated_at: "2026-10-01T00:00:00Z",
          },
        ]),
      );
    return posting instanceof Error
      ? Promise.reject(posting)
      : Promise.resolve(envelope(posting));
  });
}
function mount() {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity },
      mutations: { retry: false },
    },
  });
  client.setQueryData(["auth", "me"], mocks.user);
  render(
    <QueryClientProvider client={client}>
      <JobPostingDetail postingId={postingId} />
    </QueryClientProvider>,
  );
  return client;
}

beforeEach(() => {
  if (!HTMLElement.prototype.scrollIntoView)
    Object.defineProperty(HTMLElement.prototype, "scrollIntoView", {
      configurable: true,
      value: vi.fn(),
    });
  vi.clearAllMocks();
  mocks.get.mockReset();
  mocks.post.mockReset();
  mocks.patch.mockReset();
});

describe("JobPostingDetail", () => {
  it("edits a pending posting and sends only the changed fields", async () => {
    serve(base);
    mocks.patch.mockResolvedValue(envelope({ ...base, location: "Da Nang" }));
    mount();
    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Backend Engineer",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText("Pending review")).toBeInTheDocument();
    // The text is locked while pending, so it shows as text, not a field.
    expect(
      screen.queryByLabelText("Job description text (required)"),
    ).not.toBeInTheDocument();
    const save = screen.getByRole("button", { name: "Save changes" });
    expect(save).toBeDisabled();

    fireEvent.change(screen.getByLabelText("Location"), {
      target: { value: "Da Nang" },
    });
    await waitFor(() => expect(save).toBeEnabled());
    fireEvent.click(save);
    await waitFor(() =>
      expect(mocks.patch).toHaveBeenCalledWith(`/job-postings/${postingId}`, {
        location: "Da Nang",
      }),
    );
    await waitFor(() =>
      expect(mocks.toast).toHaveBeenCalledWith("Changes saved"),
    );
    expect(await screen.findByLabelText("Location")).toHaveValue("Da Nang");
  });

  it("locks seniority and requirements while the pending posting has a question bank", async () => {
    serve({ ...base, core_question_count: 8 });
    mount();
    expect(
      await screen.findByText(/seniority and requirements are locked/i),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Open the question bank" }),
    ).toHaveAttribute("href", `/recruiter/job-postings/${postingId}/questions`);
    expect(
      screen.queryByRole("button", { name: "Add requirement" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Suggest requirements" }),
    ).not.toBeInTheDocument();
  });

  it("lets a rejected posting be corrected and sent for review again", async () => {
    serve({
      ...base,
      status: "rejected",
      reject_reason: "Add the salary range.",
      core_question_count: 3,
    });
    mocks.post.mockResolvedValue(envelope({ ...base, status: "pending" }));
    mount();
    expect(
      await screen.findByText("Reason: Add the salary range."),
    ).toBeInTheDocument();
    expect(screen.getByText(/deletes the 3 questions/i)).toBeInTheDocument();
    expect(
      screen.getByLabelText("Job description text (required)"),
    ).toBeEnabled();

    const resend = screen.getByRole("button", {
      name: "Send for review again",
    });
    fireEvent.change(screen.getByLabelText("Minimum salary"), {
      target: { value: "1000" },
    });
    await waitFor(() => expect(resend).toBeDisabled());
    expect(
      screen.getByText("Save your changes before sending it for review."),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Minimum salary"), {
      target: { value: "" },
    });
    await waitFor(() => expect(resend).toBeEnabled());
    fireEvent.click(resend);
    await waitFor(() =>
      expect(mocks.post).toHaveBeenCalledWith(
        `/job-postings/${postingId}/resubmit`,
      ),
    );
    expect(await screen.findByText("Pending review")).toBeInTheDocument();
  });

  it("asks for confirmation again when requirements change", async () => {
    serve({ ...base, status: "rejected", reject_reason: "Too vague." });
    mocks.patch.mockResolvedValue(envelope({ ...base, status: "rejected" }));
    mount();
    fireEvent.change(await screen.findByLabelText("Requirement 1"), {
      target: { value: "Golang" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
    expect(
      await screen.findByText(
        "Confirm that you reviewed the requirements before submitting.",
      ),
    ).toBeInTheDocument();
    expect(mocks.patch).not.toHaveBeenCalled();
  });

  it("refreshes when the server says the posting changed", async () => {
    serve(base);
    mocks.patch.mockRejectedValue(
      failure(
        409,
        "JOB_POSTING_NOT_EDITABLE",
        "only a pending or rejected job posting can be edited",
      ),
    );
    mount();
    fireEvent.change(await screen.findByLabelText("Location"), {
      target: { value: "Hue" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
    expect(
      await screen.findByText(
        "only a pending or rejected job posting can be edited We refreshed it for you.",
      ),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(
        mocks.get.mock.calls.filter(([p]) => p === `/job-postings/${postingId}`)
          .length,
      ).toBeGreaterThan(1),
    );
  });

  it("shows an approved posting read-only", async () => {
    serve({ ...base, status: "open" });
    mount();
    expect(
      await screen.findByText(/was approved, so it can no longer be edited/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Save changes" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("45 minutes")).toBeInTheDocument();
  });

  it("keeps a saved voice that is no longer in the catalog", async () => {
    serve({ ...base, required_voice: { id: oldVoiceId, name: "Ben" } });
    mount();
    expect(
      await screen.findByRole("combobox", {
        name: "Interviewer voice (required)",
      }),
    ).toHaveTextContent("Ben (no longer available)");
  });

  it("shows not found for a missing or someone else's posting", async () => {
    serve(failure(404, "JOB_POSTING_NOT_FOUND", "job posting not found"));
    mount();
    expect(
      await screen.findByRole("heading", {
        name: "We couldn't find this job posting",
      }),
    ).toBeInTheDocument();
  });

  it("drops stale edits and shows the saved requirements after a lock conflict", async () => {
    let current: Posting = {
      ...base,
      status: "rejected",
      reject_reason: "Vague.",
    };
    serve(current);
    mocks.patch.mockImplementation(() => {
      current = { ...base, core_question_count: 4 };
      serve(current);
      return Promise.reject(
        failure(
          409,
          "JOB_POSTING_REQUIREMENTS_LOCKED",
          "requirements are locked",
        ),
      );
    });
    mount();
    fireEvent.change(await screen.findByLabelText("Requirement 1"), {
      target: { value: "Golang" },
    });
    fireEvent.click(
      screen.getByRole("checkbox", { name: /i reviewed these requirements/i }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
    expect(
      await screen.findByText(/seniority and requirements are locked/i),
    ).toBeInTheDocument();
    expect(screen.getByText("Go")).toBeInTheDocument();
    expect(screen.queryByText("Golang")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
  });

  it("treats a malformed posting ID as not found", async () => {
    serve(failure(400, "VALIDATION_ERROR", "invalid job posting ID"));
    mount();
    expect(
      await screen.findByRole("heading", {
        name: "We couldn't find this job posting",
      }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Try again" }),
    ).not.toBeInTheDocument();
  });
});
