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
import { JobPostingDetail } from "@/features/recruiter/job-postings/components/job-posting-detail";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  patch: vi.fn(),
  toast: vi.fn(),
  push: vi.fn(),
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
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mocks.push }),
  useSearchParams: () => new URLSearchParams(),
  usePathname: () =>
    "/recruiter/job-postings/33333333-3333-4333-8333-333333333333",
}));

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
  has_document: true,
  seniority_level: "senior",
  requirements: [
    { name: "Go", target: "programming_language", type: "required" },
  ],
  location: "Hanoi",
  employment_type: null,
  salary_min: null,
  salary_max: null,
  interview_duration: 45,
  interview_slot: null as number | null,
  required_avatar: { id: avatarId, name: "Linh" },
  required_voice: { id: voiceId, name: "Ava" },
  refinement_note: "",
  // A submitted posting with a ready bank, so its requirements are locked while pending.
  review_submitted: true,
  core_question_count: 8,
  created_at: "2026-10-12T08:00:00Z",
};
type Posting = typeof base;
const pdfUrl = "https://storage.example.com/role-cue/jd?X-Amz-Signature=abc";
const newDocumentId = "77777777-7777-4777-8777-777777777777";
function failure(status: number, code: string, message: string) {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: { success: false, error: { code, message } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}
const applicationRow = {
  id: "88888888-8888-4888-8888-888888888888",
  job_posting_id: postingId,
  job_posting_title: "Backend Engineer",
  status: "pending",
  submitted_at: "2026-10-20T08:00:00Z",
  candidate: {
    id: "99999999-9999-4999-8999-999999999999",
    name: "Linh Tran",
    email: "linh@example.com",
    image: null,
  },
  interview_eligible: false,
  interview_deadline: null,
  reject_reason: null,
  updated_at: "2026-10-20T08:00:00Z",
};
const capacityState = (over: Record<string, unknown> = {}) => ({
  interview_slot: 5,
  funded: true,
  price_per_slot: 1,
  cv_approvals: 2,
  consumed: 1,
  available_to_approve: 3,
  refundable: 4,
  posting_status: "open",
  refunded: null,
  ...over,
});
function serve(
  posting: Posting | Error,
  capacity: ReturnType<typeof capacityState> | null = null,
) {
  mocks.get.mockImplementation((path: string, config?: { params?: object }) => {
    if (path === `/job-postings/${postingId}/interview-capacity`)
      return capacity
        ? Promise.resolve(envelope(capacity))
        : Promise.reject(failure(500, "INTERNAL_ERROR", "boom"));
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
    if (path === `/job-postings/${postingId}/applications`)
      // Three applications in all, one of them awaiting a decision.
      return Promise.resolve(
        envelope({
          items: [applicationRow],
          pagination: {
            limit: 20,
            offset: 0,
            total:
              (config?.params as { status?: string } | undefined)?.status ===
              "pending"
                ? 1
                : 3,
          },
        }),
      );
    if (path === `/job-postings/${postingId}/document`)
      return Promise.resolve(
        envelope({ url: pdfUrl, expires_at: "2026-10-17T08:05:00Z" }),
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
    // The PDF is locked while pending: it can be opened, not replaced.
    expect(
      await screen.findByRole("link", { name: /open the pdf/i }),
    ).toHaveAttribute("href", pdfUrl);
    expect(
      screen.queryByRole("button", { name: /replace the pdf/i }),
    ).not.toBeInTheDocument();
    const save = screen.getByRole("button", {
      name: /Save changes|Confirm & generate questions/,
    });
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
    serve(base);
    mount();
    expect(
      await screen.findByText(
        /seniority, requirements and refinement note are locked/i,
      ),
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

  it("sends a corrected rejected posting for review from its question bank", async () => {
    serve({
      ...base,
      status: "rejected",
      reject_reason: "Add the salary range.",
      core_question_count: 3,
    });
    mount();
    expect(
      await screen.findByText("“Add the salary range.”"),
    ).toBeInTheDocument();
    expect(screen.getByText(/deletes the 3 questions/i)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /replace the pdf/i }),
    ).toBeEnabled();
    // Submission happens next to the questions, so there is one place to send it from.
    expect(screen.getByRole("link", { name: "question bank" })).toHaveAttribute(
      "href",
      `/recruiter/job-postings/${postingId}/questions`,
    );
    expect(
      screen.queryByRole("button", { name: /review again/i }),
    ).not.toBeInTheDocument();
  });

  it("saves changed requirements, generates the bank and opens it", async () => {
    const rejected = { ...base, status: "rejected", reject_reason: "Vague." };
    serve(rejected);
    const saved = {
      ...rejected,
      requirements: [
        { name: "Golang", target: "programming_language", type: "required" },
      ],
      core_question_count: 0,
    };
    mocks.patch.mockResolvedValue(envelope(saved));
    mocks.post.mockResolvedValue(
      envelope({ questions: [{ id: avatarId, content: "Explain Go." }] }),
    );
    mount();
    fireEvent.change(await screen.findByLabelText("Requirement 1"), {
      target: { value: "Golang" },
    });
    fireEvent.click(
      screen.getByRole("checkbox", { name: /i reviewed these requirements/i }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Confirm & generate questions" }),
    );
    await waitFor(() =>
      expect(mocks.post).toHaveBeenCalledWith(
        `/job-postings/${postingId}/questions/generate`,
      ),
    );
    await waitFor(() =>
      expect(mocks.push).toHaveBeenCalledWith(
        `/recruiter/job-postings/${postingId}/questions`,
      ),
    );
    expect(mocks.toast).toHaveBeenCalledWith(
      "Question bank generated",
      expect.anything(),
    );
  });

  it("offers to generate again after a reload when an earlier generation failed", async () => {
    // The posting was saved but its generation failed, so it still has no questions.
    const failed = {
      ...base,
      review_submitted: false,
      core_question_count: 0,
    };
    serve(failed);
    mocks.post.mockResolvedValue(
      envelope({ questions: [{ id: avatarId, content: "Explain Go." }] }),
    );
    mount();
    expect(
      await screen.findByText(/Confirm the requirements below to generate/),
    ).toBeInTheDocument();
    const confirm = screen.getByRole("button", {
      name: "Confirm & generate questions",
    });
    expect(confirm).toBeEnabled();
    fireEvent.click(
      screen.getByRole("checkbox", { name: /i reviewed these requirements/i }),
    );
    fireEvent.click(confirm);
    await waitFor(() =>
      expect(mocks.post).toHaveBeenCalledWith(
        `/job-postings/${postingId}/questions/generate`,
      ),
    );
    expect(mocks.patch).not.toHaveBeenCalled();
  });

  it("replaces the PDF of a rejected posting and sends only the new document", async () => {
    serve({ ...base, status: "rejected", reject_reason: "Wrong JD." });
    const corrected = "We run Kafka and Go services for payments. ".repeat(3);
    mocks.post.mockResolvedValue(
      envelope({ document_id: newDocumentId, raw_text: corrected }),
    );
    mocks.patch.mockResolvedValue(
      envelope({ ...base, status: "rejected", raw_text: corrected }),
    );
    mount();
    await screen.findByRole("button", { name: /replace the pdf/i });
    const input = document.getElementById("posting-pdf");
    if (!input) throw new Error("PDF input not found");
    fireEvent.change(input, {
      target: {
        files: [new File(["%PDF"], "fixed.pdf", { type: "application/pdf" })],
      },
    });
    expect(await screen.findByText("fixed.pdf")).toBeInTheDocument();
    expect(screen.getByLabelText("Text read from the PDF")).toHaveTextContent(
      corrected.trim(),
    );
    // The old PDF's link goes away once a new one is chosen.
    expect(
      screen.queryByRole("link", { name: /open the pdf/i }),
    ).not.toBeInTheDocument();
    // A new JD alone still needs the requirements confirmed against it.
    fireEvent.click(
      screen.getByRole("button", {
        name: /Save changes|Confirm & generate questions/,
      }),
    );
    expect(
      await screen.findByText(
        "Confirm that you reviewed the requirements before submitting.",
      ),
    ).toBeInTheDocument();
    expect(mocks.patch).not.toHaveBeenCalled();
    fireEvent.click(
      screen.getByRole("checkbox", { name: /i reviewed these requirements/i }),
    );
    fireEvent.click(
      screen.getByRole("button", {
        name: /Save changes|Confirm & generate questions/,
      }),
    );
    await waitFor(() =>
      expect(mocks.patch).toHaveBeenCalledWith(`/job-postings/${postingId}`, {
        document_id: newDocumentId,
      }),
    );
  });

  it("clears the confirmation when a requirement is edited after ticking", async () => {
    serve({ ...base, status: "rejected", reject_reason: "Too vague." });
    mount();
    fireEvent.change(await screen.findByLabelText("Requirement 1"), {
      target: { value: "Golang" },
    });
    const confirm = () =>
      screen.getByRole("checkbox", { name: /i reviewed these requirements/i });
    fireEvent.click(confirm());
    expect(confirm()).toBeChecked();
    fireEvent.change(screen.getByLabelText("Requirement 1"), {
      target: { value: "Rust" },
    });
    await waitFor(() => expect(confirm()).not.toBeChecked());
    fireEvent.click(
      screen.getByRole("button", {
        name: /Save changes|Confirm & generate questions/,
      }),
    );
    expect(
      await screen.findByText(
        "Confirm that you reviewed the requirements before submitting.",
      ),
    ).toBeInTheDocument();
    expect(mocks.patch).not.toHaveBeenCalled();
  });

  it("asks for confirmation again when requirements change", async () => {
    serve({ ...base, status: "rejected", reject_reason: "Too vague." });
    mocks.patch.mockResolvedValue(envelope({ ...base, status: "rejected" }));
    mount();
    fireEvent.change(await screen.findByLabelText("Requirement 1"), {
      target: { value: "Golang" },
    });
    fireEvent.click(
      screen.getByRole("button", {
        name: /Save changes|Confirm & generate questions/,
      }),
    );
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
    fireEvent.click(
      screen.getByRole("button", {
        name: /Save changes|Confirm & generate questions/,
      }),
    );
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

  it("tells the Recruiter to fund an approved posting to open intake", async () => {
    serve({ ...base, status: "open", interview_slot: null });
    mount();
    expect(
      await screen.findByText(
        /Fund interview slots to open intake to candidates/i,
      ),
    ).toBeInTheDocument();
  });

  it("does not repeat the funding prompt once the posting is funded", async () => {
    serve({ ...base, status: "open", interview_slot: 5 });
    mount();
    expect(
      await screen.findByText(
        /Approved, so the posting can no longer be edited/i,
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/Fund interview slots to open intake/i),
    ).not.toBeInTheDocument();
  });

  it("shows an approved posting read-only", async () => {
    serve({ ...base, status: "open" });
    mount();
    expect(
      await screen.findByText(
        /Approved, so the posting can no longer be edited/i,
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", {
        name: /Save changes|Confirm & generate questions/,
      }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("45 minutes")).toBeInTheDocument();
  });

  it("links to the evaluation criteria and shows the interview slots", async () => {
    serve({ ...base, interview_slot: 5 });
    mount();
    expect(
      await screen.findByRole("link", { name: "Evaluation" }),
    ).toHaveAttribute(
      "href",
      `/recruiter/job-postings/${postingId}/evaluation`,
    );
    expect(screen.getByText("Interview slots:")).toHaveTextContent("5");
  });

  it("says when interview slots are not funded yet", async () => {
    serve(base);
    mount();
    expect(await screen.findByText("Interview slots:")).toHaveTextContent(
      "not funded yet",
    );
  });

  it("does not list applications before the posting is approved", async () => {
    serve(base);
    mount();
    expect(
      await screen.findByText(
        "Candidates can apply after an administrator approves this posting and you fund interview slots, which opens intake.",
      ),
    ).toBeInTheDocument();
    expect(
      mocks.get.mock.calls.some(([path]) =>
        String(path).endsWith("/applications"),
      ),
    ).toBe(false);
  });

  it("summarises the applications of an approved posting and links to the review inbox", async () => {
    serve({ ...base, status: "open", interview_slot: 5 });
    mount();
    const summary = await screen.findByRole("region", {
      name: "Applications",
    });
    expect(
      await within(summary).findByRole("link", {
        name: "Review 1 awaiting CV",
      }),
    ).toHaveAttribute(
      "href",
      `/recruiter/applications?status=pending&posting=${postingId}`,
    );
    expect(summary).toHaveTextContent("3 applications, 1 awaiting review");
    // The list itself lives in the inbox, so no row is rendered here.
    expect(screen.queryByText("Linh Tran")).not.toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledWith(
      `/job-postings/${postingId}/applications`,
      { params: { limit: 20, offset: 0 }, signal: expect.any(AbortSignal) },
    );
  });

  it("shows the posting's sections as tabs and keeps Applications off until it takes applications", async () => {
    serve(base);
    mount();
    const tabs = await screen.findByRole("navigation", {
      name: "Posting sections",
    });
    expect(
      within(tabs).getByRole("link", { name: "Overview" }),
    ).toHaveAttribute("aria-current", "page");
    expect(
      within(tabs).getByRole("link", { name: "Questions" }),
    ).toHaveAttribute("href", `/recruiter/job-postings/${postingId}/questions`);
    const applications = within(tabs).getByRole("link", {
      name: /Applications/,
    });
    expect(applications).toHaveAttribute("aria-disabled", "true");
    expect(applications).not.toHaveAttribute("href");
    expect(applications).toHaveTextContent(
      /available once an administrator approves the posting/,
    );
  });

  it("opens the review inbox for this posting from the Applications tab once approved", async () => {
    serve({ ...base, status: "open", interview_slot: 5 });
    mount();
    const tabs = await screen.findByRole("navigation", {
      name: "Posting sections",
    });
    expect(
      within(tabs).getByRole("link", { name: "Applications" }),
    ).toHaveAttribute("href", `/recruiter/applications?posting=${postingId}`);
  });

  it.each([
    [
      "a bank ready to send",
      { ...base, review_submitted: false },
      "Review & submit",
      `/recruiter/job-postings/${postingId}/questions`,
    ],
    [
      "a posting without a bank",
      { ...base, review_submitted: false, core_question_count: 0 },
      "Review requirements",
      "#posting-requirements",
    ],
    [
      "a rejected posting",
      { ...base, status: "rejected", reject_reason: "Vague." },
      "Edit and resubmit",
      `/recruiter/job-postings/${postingId}/questions`,
    ],
    [
      "an approved posting that is not funded",
      { ...base, status: "open", interview_slot: null },
      "Fund & publish",
      "#interview-capacity",
    ],
    [
      "a live posting",
      { ...base, status: "open", interview_slot: 5 },
      "Review applications",
      `/recruiter/applications?posting=${postingId}`,
    ],
  ])("offers one next action for %s", async (_name, posting, label, href) => {
    serve(posting as Posting);
    mount();
    await screen.findByRole("heading", { level: 1, name: "Backend Engineer" });
    const link = screen.getByRole("link", { name: label });
    expect(link).toHaveAttribute("href", href);
    // The action is the header's single primary link, so the label is not repeated elsewhere.
    expect(screen.getAllByRole("link", { name: label })).toHaveLength(1);
  });

  it("offers no next action while the posting waits for review or after it ended", async () => {
    serve(base);
    mount();
    await screen.findByRole("heading", { level: 1, name: "Backend Engineer" });
    for (const label of [
      "Review & submit",
      "Edit and resubmit",
      "Fund & publish",
      "Review applications",
    ])
      expect(
        screen.queryByRole("link", { name: label }),
      ).not.toBeInTheDocument();
  });

  it.each([
    ["Waiting for admin review", base, null],
    [
      "Rejected: see the reason below",
      { ...base, status: "rejected", reject_reason: "Vague." },
      null,
    ],
    [
      "Approved: fund interview slots to publish",
      { ...base, status: "open", interview_slot: null },
      null,
    ],
    [
      "2 of 5 interview slots approved",
      { ...base, status: "open", interview_slot: 5 },
      capacityState(),
    ],
    [
      "Intake closed: 2 of 5 slots approved",
      { ...base, status: "intake_closed", interview_slot: 5 },
      capacityState({ posting_status: "intake_closed" }),
    ],
    [
      "Intake closed: all 5 slots approved",
      { ...base, status: "intake_closed", interview_slot: 5 },
      capacityState({ posting_status: "intake_closed", cv_approvals: 5 }),
    ],
    [
      "Closed: 4 coins from unused slots returned to your wallet",
      { ...base, status: "closed", interview_slot: 5 },
      capacityState({ posting_status: "closed", refunded: 4 }),
    ],
    [
      "Closed: no unused slots to return",
      { ...base, status: "closed", interview_slot: 5 },
      capacityState({ posting_status: "closed", refunded: 0 }),
    ],
  ])("states where the posting stands: %s", async (line, posting, capacity) => {
    serve(posting as Posting, capacity);
    mount();
    expect(await screen.findByText(line)).toBeInTheDocument();
  });

  it("leaves the slot figures out of the header when the capacity cannot be read", async () => {
    serve({ ...base, status: "open", interview_slot: 5 }, null);
    mount();
    await screen.findByRole("heading", { level: 1, name: "Backend Engineer" });
    expect(screen.queryByText(/interview slots approved/)).toBeNull();
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
    fireEvent.click(
      screen.getByRole("button", {
        name: /Save changes|Confirm & generate questions/,
      }),
    );
    expect(
      await screen.findByText(
        /seniority, requirements and refinement note are locked/i,
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Go")).toBeInTheDocument();
    expect(screen.queryByText("Golang")).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", {
        name: /Save changes|Confirm & generate questions/,
      }),
    ).toBeDisabled();
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
