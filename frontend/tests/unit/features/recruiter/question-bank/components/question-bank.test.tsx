import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { QuestionBank } from "@/features/recruiter/question-bank/components/question-bank";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  getPosting: vi.fn(),
  getWeights: vi.fn(),
  post: vi.fn(),
  patch: vi.fn(),
  delete: vi.fn(),
  toast: vi.fn(),
  user: {
    id: "11111111-1111-4111-8111-111111111111",
    full_name: "Rec Ruiter",
    email: "rec@example.com",
    role: "recruiter" as const,
    email_verified: true,
    is_locked: false,
    image: null,
  },
}));
vi.mock("@/lib/api/client", () => ({
  apiClient: {
    // Review reads both posting metadata and the effective evaluation weights.
    get: (path: string, ...rest: unknown[]) =>
      path.endsWith("/questions")
        ? mocks.get(path, ...rest)
        : path.endsWith("/evaluation-weights")
          ? mocks.getWeights(path, ...rest)
          : mocks.getPosting(path, ...rest),
    post: mocks.post,
    patch: mocks.patch,
    delete: mocks.delete,
  },
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));
vi.mock("goey-toast", () => ({ gooeyToast: { success: mocks.toast } }));

const postingId = "33333333-3333-4333-8333-333333333333";
const q1 = {
  id: "44444444-4444-4444-8444-444444444441",
  content: "Explain Go channels.",
};
const q2 = {
  id: "44444444-4444-4444-8444-444444444442",
  content: "How do you index a table?",
};
const bank = (overrides: Record<string, unknown> = {}) => ({
  questions: [q1, q2],
  posting_status: "pending",
  editable: true,
  max_question_length: 1000,
  review_submitted: false,
  generation_size: 10,
  ...overrides,
});
const posting = {
  id: postingId,
  title: "Backend Engineer",
  status: "pending",
  reject_reason: null,
  raw_text: "A technical role",
  has_document: true,
  seniority_level: "senior",
  requirements: [],
  refinement_note: "",
  review_submitted: false,
  location: null,
  employment_type: null,
  salary_min: null,
  salary_max: null,
  interview_duration: null,
  interview_slot: null,
  required_avatar: null,
  required_voice: null,
  core_question_count: 2,
  created_at: "2026-10-12T08:00:00Z",
};
const envelope = (data: unknown) => ({ data: { success: true, data } });
function failure(status: number, code: string, message: string) {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: { success: false, error: { code, message } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
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
  renderWithIntl(
    <QueryClientProvider client={client}>
      <QuestionBank postingId={postingId} />
    </QueryClientProvider>,
  );
  return client;
}
const path = `/job-postings/${postingId}/questions`;

async function confirmReview(
  review: HTMLElement,
  name = "Submit for approval",
) {
  const confirm = within(review).getByRole("button", { name });
  // The effective weights are fetched when review opens; wait until the user can confirm.
  await waitFor(() => expect(confirm).toBeEnabled());
  fireEvent.click(confirm);
}

beforeEach(() => {
  vi.clearAllMocks();
  for (const fn of [mocks.get, mocks.post, mocks.patch, mocks.delete])
    fn.mockReset();
  mocks.getPosting.mockResolvedValue(envelope(posting));
  mocks.getWeights.mockReset();
  mocks.getWeights.mockResolvedValue(
    envelope({
      configured: false,
      using_defaults: true,
      editable: true,
      posting_status: "pending",
      total: 100,
      weights: [
        {
          metric_id: "77777777-7777-4777-8777-777777777777",
          name: "Technical correctness",
          percentage: 100,
        },
      ],
    }),
  );
});

describe("QuestionBank", () => {
  it("lists the questions of an editable bank", async () => {
    mocks.get.mockResolvedValue(envelope(bank()));
    mount();
    expect(
      await screen.findByRole("heading", { level: 1, name: "Question bank" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("Explain Go channels.")).toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledWith(path, expect.anything());
    expect(
      within(screen.getByRole("region", { name: "Questions" })).getByText(
        "2 questions",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Edit question 1" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("form", { name: "Add a question" }),
    ).toBeInTheDocument();
  });

  it("is read-only with the reason when the posting is no longer pending", async () => {
    mocks.get.mockResolvedValue(
      envelope(bank({ posting_status: "open", editable: false })),
    );
    mount();
    expect(
      await screen.findByText(/so the question bank is read-only/i),
    ).toHaveTextContent("This posting is approved");
    expect(
      screen.queryByRole("button", { name: /edit question/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("form", { name: "Add a question" }),
    ).not.toBeInTheDocument();
  });

  it("generates an empty bank once and shows the progress", async () => {
    // An empty bank is generated, not edited by hand.
    mocks.get.mockResolvedValue(
      envelope(bank({ questions: [], editable: false })),
    );
    let resolve: (value: unknown) => void = () => {};
    mocks.post.mockReturnValue(new Promise((r) => (resolve = r)));
    mount();
    fireEvent.click(
      await screen.findByRole("button", { name: "Generate questions" }),
    );
    expect(
      await screen.findByRole("button", { name: "Generating questions…" }),
    ).toBeDisabled();
    expect(mocks.post).toHaveBeenCalledWith(
      `/job-postings/${postingId}/questions/generate`,
    );
    resolve(envelope({ questions: [q1] }));
    expect(await screen.findByText("Explain Go channels.")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /generate/i }),
    ).not.toBeInTheDocument();
  });

  it("shows the server message when generation fails", async () => {
    mocks.get.mockResolvedValue(
      envelope(bank({ questions: [], editable: false })),
    );
    mocks.post.mockRejectedValue(
      failure(
        409,
        "REQUIREMENTS_NOT_CONFIRMED",
        "confirm the job description requirements before generating questions",
      ),
    );
    mount();
    fireEvent.click(
      await screen.findByRole("button", { name: "Generate questions" }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "confirm the job description requirements",
    );
    expect(
      screen.getByRole("button", { name: "Retry generating questions" }),
    ).toBeEnabled();
  });

  it("offers no generation when the server has it switched off", async () => {
    mocks.get.mockResolvedValue(
      envelope(bank({ questions: [], generation_size: 0 })),
    );
    mount();
    expect(
      await screen.findByText(/automatic generation isn.t available/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Retry generating questions" }),
    ).not.toBeInTheDocument();
  });

  it("adds a question with exactly the body Go accepts", async () => {
    mocks.get.mockResolvedValue(envelope(bank()));
    const added = {
      id: "44444444-4444-4444-8444-444444444443",
      content: "What is a goroutine leak?",
    };
    mocks.post.mockResolvedValue(envelope(added));
    mount();
    const form = await screen.findByRole("form", { name: "Add a question" });
    fireEvent.change(within(form).getByLabelText("Add a question"), {
      target: { value: "  What is a goroutine leak?  " },
    });
    fireEvent.click(
      await within(form).findByRole("button", { name: "Add question" }),
    );
    await waitFor(() =>
      expect(mocks.post).toHaveBeenCalledWith(path, {
        content: "What is a goroutine leak?",
      }),
    );
    expect(
      await screen.findByText("What is a goroutine leak?"),
    ).toBeInTheDocument();
    expect(
      within(screen.getByRole("region", { name: "Questions" })).getByText(
        "3 questions",
      ),
    ).toBeInTheDocument();
  });

  it("uses the server limit for the length check", async () => {
    mocks.get.mockResolvedValue(envelope(bank({ max_question_length: 10 })));
    mount();
    const form = await screen.findByRole("form", { name: "Add a question" });
    fireEvent.change(within(form).getByLabelText("Add a question"), {
      target: { value: "This question is too long" },
    });
    expect(await within(form).findByRole("alert")).toHaveTextContent(
      "10 characters or fewer",
    );
    expect(
      within(form).getByRole("button", { name: "Add question" }),
    ).toBeDisabled();
  });

  it("keeps the edit draft when saving fails", async () => {
    mocks.get.mockResolvedValue(envelope(bank()));
    mocks.patch.mockRejectedValue(
      failure(500, "INTERNAL_ERROR", "something went wrong"),
    );
    mount();
    fireEvent.click(
      await screen.findByRole("button", { name: "Edit question 1" }),
    );
    const field = screen.getByRole("textbox", { name: "Edit question 1" });
    fireEvent.change(field, {
      target: { value: "Explain buffered channels." },
    });
    fireEvent.click(
      await screen.findByRole("button", { name: "Save question" }),
    );
    await waitFor(() =>
      expect(mocks.patch).toHaveBeenCalledWith(`${path}/${q1.id}`, {
        content: "Explain buffered channels.",
      }),
    );
    expect(await screen.findByText("something went wrong")).toBeInTheDocument();
    expect(
      screen.getByRole("textbox", { name: "Edit question 1" }),
    ).toHaveValue("Explain buffered channels.");
  });

  it("shows the backend conflict when a used question can't be deleted", async () => {
    mocks.get.mockResolvedValue(envelope(bank()));
    mocks.delete.mockRejectedValue(
      failure(
        409,
        "CORE_QUESTION_IN_USE",
        "this question was already used in an interview",
      ),
    );
    mount();
    fireEvent.click(
      await screen.findByRole("button", { name: "Delete question 2" }),
    );
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Delete question" }),
    );
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "already used in an interview",
    );
    expect(screen.getByText("How do you index a table?")).toBeInTheDocument();
  });

  it("deletes a question after confirmation", async () => {
    mocks.get.mockResolvedValue(envelope(bank()));
    mocks.delete.mockResolvedValue({ status: 204 });
    mount();
    fireEvent.click(
      await screen.findByRole("button", { name: "Delete question 2" }),
    );
    fireEvent.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Delete question",
      }),
    );
    await waitFor(() =>
      expect(
        screen.queryByText("How do you index a table?"),
      ).not.toBeInTheDocument(),
    );
    expect(mocks.delete).toHaveBeenCalledWith(`${path}/${q2.id}`);
  });

  it("refreshes into read-only when the posting was locked meanwhile", async () => {
    mocks.get
      .mockResolvedValueOnce(envelope(bank()))
      .mockResolvedValue(
        envelope(bank({ posting_status: "open", editable: false })),
      );
    mocks.post.mockRejectedValue(
      failure(
        409,
        "QUESTION_BANK_LOCKED",
        "the question bank can only be changed while the job posting is pending",
      ),
    );
    mount();
    const form = await screen.findByRole("form", { name: "Add a question" });
    fireEvent.change(within(form).getByLabelText("Add a question"), {
      target: { value: "New one?" },
    });
    fireEvent.click(
      await within(form).findByRole("button", { name: "Add question" }),
    );
    expect(
      await screen.findByText(
        /can only be changed while the job posting is pending/,
      ),
    ).toBeInTheDocument();
    expect(
      await screen.findByText(/so the question bank is read-only/i),
    ).toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledTimes(2);
  });

  it("shows access denied for a non-owner or wrong role", async () => {
    mocks.get.mockRejectedValue(
      failure(
        403,
        "FORBIDDEN",
        "only recruiters can manage a job posting's question bank",
      ),
    );
    mount();
    expect(
      await screen.findByRole("heading", {
        name: /don't have access to this question bank/i,
      }),
    ).toBeInTheDocument();
  });

  it("shows a missing posting without a retry", async () => {
    mocks.get.mockRejectedValue(
      failure(404, "JOB_POSTING_NOT_FOUND", "job posting not found"),
    );
    mount();
    expect(
      await screen.findByRole("heading", {
        name: "This job posting doesn't exist",
      }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Try again" }),
    ).not.toBeInTheDocument();
  });

  it("retries after a load failure", async () => {
    mocks.get
      .mockRejectedValueOnce(failure(500, "INTERNAL_ERROR", "boom"))
      .mockResolvedValue(envelope(bank()));
    mount();
    fireEvent.click(await screen.findByRole("button", { name: "Try again" }));
    expect(await screen.findByText("Explain Go channels.")).toBeInTheDocument();
  });
});

describe("why submission is unavailable", () => {
  const submit = async () =>
    screen.findByRole("button", { name: "Submit for approval" });

  it("says a bank needs a question before it can be sent", async () => {
    mocks.get.mockResolvedValue(envelope(bank({ questions: [] })));
    mount();
    const button = await submit();
    expect(button).toBeDisabled();
    const reason = screen.getByText(
      "Generate or add at least one question to submit.",
    );
    expect(button).toHaveAttribute("aria-describedby", reason.id);
  });

  it("names the question still being edited", async () => {
    mocks.get.mockResolvedValue(envelope(bank()));
    mount();
    const button = await submit();
    expect(button).toBeEnabled();
    expect(screen.queryByText(/to submit\.$/)).not.toBeInTheDocument();
    expect(button).not.toHaveAttribute("aria-describedby");

    fireEvent.click(screen.getByRole("button", { name: "Edit question 1" }));
    expect(button).toBeDisabled();
    expect(
      screen.getByText(
        "Save or cancel the question you are editing to submit.",
      ),
    ).toBeInTheDocument();
    expect(button).toHaveAttribute("aria-describedby", "submit-block-reason");

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(button).toBeEnabled();
  });

  it("names the new question that was started but not added", async () => {
    mocks.get.mockResolvedValue(envelope(bank()));
    mount();
    const button = await submit();
    const form = await screen.findByRole("form", { name: "Add a question" });
    fireEvent.change(within(form).getByLabelText("Add a question"), {
      target: { value: "Describe a hard bug you fixed." },
    });
    await waitFor(() => expect(button).toBeDisabled());
    expect(
      screen.getByText("Add or discard the question you started to submit."),
    ).toBeInTheDocument();
  });
});

describe("submission follows question review", () => {
  it("makes evaluation criteria accessible before the first submission", async () => {
    mocks.get.mockResolvedValue(envelope(bank()));
    mount();

    expect(
      await screen.findByRole("link", { name: "Evaluation" }),
    ).toHaveAttribute(
      "href",
      `/recruiter/job-postings/${postingId}/evaluation`,
    );
    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("lets the recruiter inspect and cancel the final review without submitting", async () => {
    mocks.get.mockResolvedValue(envelope(bank()));
    mount();

    fireEvent.click(
      await screen.findByRole("button", { name: "Submit for approval" }),
    );
    const review = await screen.findByRole("dialog", {
      name: "Review & submit",
    });
    expect(within(review).getByText("Backend Engineer")).toBeInTheDocument();
    expect(mocks.post).not.toHaveBeenCalled();
    fireEvent.click(within(review).getByRole("button", { name: "Cancel" }));

    await waitFor(() =>
      expect(
        screen.queryByRole("dialog", { name: "Review & submit" }),
      ).not.toBeInTheDocument(),
    );
    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("refreshes into Resubmit when the Admin rejected the posting meanwhile", async () => {
    mocks.get
      .mockResolvedValueOnce(envelope(bank()))
      .mockResolvedValue(envelope(bank({ posting_status: "rejected" })));
    mocks.post.mockRejectedValue(
      failure(
        409,
        "JOB_POSTING_NOT_PENDING",
        "only a pending job posting can be submitted",
      ),
    );
    mount();
    fireEvent.click(
      await screen.findByRole("button", { name: "Submit for approval" }),
    );
    const review = await screen.findByRole("dialog", {
      name: "Review & submit",
    });
    await confirmReview(review);
    expect(
      await screen.findByRole("button", { name: "Resubmit for approval" }),
    ).toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledTimes(2);
  });

  it("submits a generated bank only after the recruiter clicks submit", async () => {
    mocks.get.mockResolvedValue(envelope(bank()));
    mocks.post.mockResolvedValue(
      envelope({ ...posting, review_submitted: true }),
    );
    mount();
    const send = await screen.findByRole("button", {
      name: "Submit for approval",
    });
    expect(mocks.post).not.toHaveBeenCalled();
    fireEvent.click(send);
    const review = await screen.findByRole("dialog", {
      name: "Review & submit",
    });
    expect(within(review).getByText("Backend Engineer")).toBeInTheDocument();
    expect(mocks.post).not.toHaveBeenCalled();
    await confirmReview(review);
    await waitFor(() =>
      expect(mocks.post).toHaveBeenCalledWith(
        `/job-postings/${postingId}/submit`,
      ),
    );
    expect(
      await screen.findByText(/^Submitted for approval\./),
    ).toBeInTheDocument();
  });
  it("resubmits a rejected posting only after confirming the final review", async () => {
    mocks.get.mockResolvedValue(envelope(bank({ posting_status: "rejected" })));
    mocks.getPosting.mockResolvedValue(
      envelope({
        ...posting,
        status: "rejected",
        reject_reason: "Clarify the role.",
      }),
    );
    mocks.post.mockResolvedValue(
      envelope({ ...posting, review_submitted: true }),
    );
    mount();

    fireEvent.click(
      await screen.findByRole("button", { name: "Resubmit for approval" }),
    );
    const review = await screen.findByRole("dialog", {
      name: "Review & submit",
    });
    expect(mocks.post).not.toHaveBeenCalled();
    await confirmReview(review, "Resubmit for approval");

    await waitFor(() =>
      expect(mocks.post).toHaveBeenCalledWith(
        `/job-postings/${postingId}/resubmit`,
      ),
    );
  });
  it("blocks confirmation when evaluation criteria cannot be loaded", async () => {
    mocks.get.mockResolvedValue(envelope(bank()));
    mocks.getWeights.mockRejectedValue(
      failure(503, "SERVICE_UNAVAILABLE", "Evaluation unavailable"),
    );
    mount();
    fireEvent.click(
      await screen.findByRole("button", { name: "Submit for approval" }),
    );
    const review = await screen.findByRole("dialog", {
      name: "Review & submit",
    });

    expect(await within(review).findByRole("alert")).toHaveTextContent(
      "We couldn't load the criteria.",
    );
    expect(
      within(review).getByRole("button", { name: "Submit for approval" }),
    ).toBeDisabled();
    expect(mocks.post).not.toHaveBeenCalled();
  });
  it("blocks submission while a question draft is unsaved", async () => {
    mocks.get.mockResolvedValue(envelope(bank()));
    mount();
    const send = await screen.findByRole("button", {
      name: "Submit for approval",
    });
    fireEvent.change(
      within(screen.getByRole("form", { name: "Add a question" })).getByRole(
        "textbox",
      ),
      { target: { value: "Unsaved question" } },
    );
    await waitFor(() => expect(send).toBeDisabled());
    expect(mocks.post).not.toHaveBeenCalled();
  });
  it("withdraws the submission when the last question is deleted", async () => {
    mocks.get.mockResolvedValue(
      envelope(bank({ questions: [q1], review_submitted: true })),
    );
    mocks.delete.mockResolvedValue({ status: 204 });
    mount();
    fireEvent.click(
      await screen.findByRole("button", { name: "Delete question 1" }),
    );
    fireEvent.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Delete question",
      }),
    );
    expect(
      await screen.findByRole("button", { name: "Submit for approval" }),
    ).toBeDisabled();
    expect(
      screen.getByText(/needs at least one question before you can submit it/),
    ).toBeInTheDocument();
    expect(mocks.toast).toHaveBeenCalledWith("Question deleted", {
      description: expect.stringMatching(/left the review queue/),
    });
  });
});
