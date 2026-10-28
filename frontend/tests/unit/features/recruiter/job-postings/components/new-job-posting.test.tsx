import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { NewJobPosting } from "@/features/recruiter/job-postings/components/new-job-posting";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  replace: vi.fn(),
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
  apiClient: { get: mocks.get, post: mocks.post },
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mocks.replace }),
}));
vi.mock("goey-toast", () => ({ gooeyToast: { success: mocks.toast } }));

const avatarId = "44444444-4444-4444-8444-444444444444";
const voiceId = "55555555-5555-4555-8555-555555555555";
const postingId = "33333333-3333-4333-8333-333333333333";
const jdText = "We build Go services on PostgreSQL for payments. ".repeat(3);
const envelope = <T,>(data: T) => ({ data: { success: true, data } });
const pageOf = <T,>(items: T[]) =>
  envelope({
    items,
    pagination: { limit: 100, offset: 0, total: items.length },
  });
const avatar = {
  id: avatarId,
  name: "Linh",
  personality: "Calm",
  created_at: "2026-10-01T00:00:00Z",
  updated_at: "2026-10-01T00:00:00Z",
};
const created = {
  id: postingId,
  title: "Backend Engineer",
  status: "pending",
  reject_reason: null,
  raw_text: jdText,
  has_document: true,
  seniority_level: "senior",
  requirements: [
    { name: "Go", target: "programming_language", type: "required" },
  ],
  location: null,
  employment_type: null,
  salary_min: null,
  salary_max: null,
  interview_duration: null,
  interview_slot: null,
  required_avatar: { id: avatarId, name: "Linh" },
  required_voice: { id: voiceId, name: "Ava" },
  refinement_note: "",
  review_submitted: false,
  core_question_count: 0,
  created_at: "2026-10-12T08:00:00Z",
};
// The body of POST /job-postings/:id/questions/generate.
const generated = {
  questions: [
    { id: "88888888-8888-4888-8888-888888888888", content: "Explain Go." },
  ],
};
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
  render(
    <QueryClientProvider client={client}>
      <NewJobPosting />
    </QueryClientProvider>,
  );
  return client;
}
async function choose(label: string, option: string) {
  fireEvent.keyDown(await screen.findByRole("combobox", { name: label }), {
    key: "ArrowDown",
  });
  fireEvent.keyDown(await screen.findByRole("option", { name: option }), {
    key: "Enter",
  });
}
const documentId = "66666666-6666-4666-8666-666666666666";
// The whole-form flows click through every step; on a busy machine they outlast the 5s default.
const LONG_FLOW_MS = 15_000;
const pdf = (name = "backend.pdf", size = 1024) =>
  new File([new Uint8Array(size)], name, { type: "application/pdf" });
const pickFile = (file: File) => {
  const input = document.getElementById("posting-pdf");
  if (!input) throw new Error("PDF input not found");
  fireEvent.change(input, { target: { files: [file] } });
};
const uploaded = envelope({ document_id: documentId, raw_text: jdText });
async function uploadPdf() {
  pickFile(pdf());
  expect(
    await screen.findByLabelText("Text read from the PDF"),
  ).toHaveTextContent(jdText.trim());
}
async function fillAndExtract() {
  await uploadPdf();
  fireEvent.click(screen.getByRole("button", { name: "Suggest requirements" }));
  expect(await screen.findByLabelText("Requirement 1")).toHaveValue("Go");
}

beforeEach(() => {
  // Radix Select scrolls the chosen option into view; jsdom has no layout.
  if (!HTMLElement.prototype.scrollIntoView)
    Object.defineProperty(HTMLElement.prototype, "scrollIntoView", {
      configurable: true,
      value: vi.fn(),
    });
  vi.clearAllMocks();
  mocks.get.mockReset();
  mocks.post.mockReset();
  mocks.get.mockImplementation((path: string) =>
    Promise.resolve(
      path === "/voices"
        ? pageOf([{ id: voiceId, name: "Ava" }])
        : pageOf([avatar]),
    ),
  );
});

describe("NewJobPosting", () => {
  it(
    "uploads, confirms requirements and opens the generated bank before submission",
    async () => {
      mocks.post.mockImplementation((path: string) =>
        Promise.resolve(
          path === "/job-postings/documents"
            ? uploaded
            : path === "/job-postings/extract"
              ? envelope({
                  title: "Backend Engineer",
                  seniority_level: "senior",
                  requirements: [
                    {
                      name: "Go",
                      target: "programming_language",
                      type: "required",
                    },
                  ],
                })
              : envelope(
                  path.endsWith("/questions/generate") ? generated : created,
                ),
        ),
      );
      const client = mount();
      expect(
        screen.getByRole("button", { name: "Suggest requirements" }),
      ).toBeDisabled();
      await fillAndExtract();
      const [path, body] = mocks.post.mock.calls[0] as [string, FormData];
      expect(path).toBe("/job-postings/documents");
      expect((body.get("file") as File).name).toBe("backend.pdf");
      expect(screen.getByText("backend.pdf")).toBeInTheDocument();
      expect(mocks.post).toHaveBeenCalledWith("/job-postings/extract", {
        raw_text: jdText,
      });
      expect(screen.getByLabelText("Title (required)")).toHaveValue(
        "Backend Engineer",
      );
      expect(
        screen.getByRole("combobox", { name: "Seniority (required)" }),
      ).toHaveTextContent("Senior");

      await choose("Interviewer model (required)", "Linh");
      await choose("Interviewer voice (required)", "Ava");
      fireEvent.change(screen.getByLabelText("Interview length (minutes)"), {
        target: { value: "45" },
      });

      fireEvent.click(
        screen.getByRole("button", { name: "Confirm & generate questions" }),
      );
      expect(
        await screen.findByText(
          "Confirm that you reviewed the requirements before submitting.",
        ),
      ).toBeInTheDocument();
      expect(mocks.post).toHaveBeenCalledTimes(2);

      fireEvent.click(
        screen.getByRole("checkbox", {
          name: /i reviewed these requirements/i,
        }),
      );
      fireEvent.click(
        screen.getByRole("button", { name: "Confirm & generate questions" }),
      );
      await waitFor(() =>
        expect(mocks.post).toHaveBeenCalledWith("/job-postings", {
          title: "Backend Engineer",
          document_id: documentId,
          refinement_note: "",
          seniority_level: "senior",
          requirements: [
            { name: "Go", target: "programming_language", type: "required" },
          ],
          location: null,
          employment_type: null,
          salary_min: null,
          salary_max: null,
          interview_duration: 45,
          required_avatar_id: avatarId,
          required_voice_id: voiceId,
        }),
      );
      await waitFor(() =>
        expect(mocks.replace).toHaveBeenCalledWith(
          `/recruiter/job-postings/${postingId}/questions`,
        ),
      );
      expect(mocks.post).toHaveBeenCalledWith(
        `/job-postings/${postingId}/questions/generate`,
      );
      expect(mocks.post).not.toHaveBeenCalledWith(
        `/job-postings/${postingId}/submit`,
        expect.anything(),
      );
      expect(
        client.getQueryData([
          "recruiter-job-postings",
          mocks.user.id,
          "detail",
          postingId,
        ]),
      ).toEqual(created);
    },
    LONG_FLOW_MS,
  );

  it("keeps the PDF and explains when extraction fails", async () => {
    mocks.post.mockImplementation((path: string) =>
      path === "/job-postings/documents"
        ? Promise.resolve(uploaded)
        : Promise.reject(
            failure(
              502,
              "EXTRACTION_FAILED",
              "Job description extraction failed.",
            ),
          ),
    );
    mount();
    await uploadPdf();
    fireEvent.click(
      screen.getByRole("button", { name: "Suggest requirements" }),
    );
    expect(
      await screen.findByText(
        /job description extraction failed\. your pdf is still uploaded/i,
      ),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Text read from the PDF")).toHaveTextContent(
      jdText.trim(),
    );
    expect(
      screen.getByRole("button", { name: "Add requirement" }),
    ).toBeEnabled();
  });

  it("refuses non-PDF and oversized files without uploading", async () => {
    mount();
    pickFile(new File(["hello"], "jd.txt", { type: "text/plain" }));
    expect(await screen.findByText("Choose a PDF file.")).toBeInTheDocument();
    pickFile(pdf("big.pdf", 5 * 1024 * 1024 + 1));
    expect(
      await screen.findByText("That PDF is too large. Choose one up to 5 MB."),
    ).toBeInTheDocument();
    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("shows the server's reason when a PDF can't be read", async () => {
    mocks.post.mockRejectedValue(
      failure(
        400,
        "INVALID_PDF",
        "This PDF has no readable text. Scanned PDFs need OCR.",
      ),
    );
    mount();
    pickFile(pdf());
    expect(
      await screen.findByText(
        "This PDF has no readable text. Scanned PDFs need OCR.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByLabelText("Text read from the PDF"),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Upload the job description PDF" }),
    ).toBeEnabled();
  });

  it("shows field errors before calling the API", async () => {
    mount();
    fireEvent.click(
      screen.getByRole("button", { name: "Confirm & generate questions" }),
    );
    expect(
      await screen.findByText("Upload the job description as a PDF."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Choose the interviewer model candidates will meet."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Choose the interviewer voice."),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Title (required)")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("sends one request on a double click and keeps the draft when the server refuses", async () => {
    let reject: (reason: unknown) => void = () => {};
    mocks.post.mockImplementation((path: string) =>
      path === "/job-postings/documents"
        ? Promise.resolve(uploaded)
        : path === "/job-postings/extract"
          ? Promise.resolve(
              envelope({
                title: "Backend Engineer",
                seniority_level: "mid",
                requirements: [
                  {
                    name: "Go",
                    target: "programming_language",
                    type: "required",
                  },
                ],
              }),
            )
          : new Promise((_, fail) => {
              reject = fail;
            }),
    );
    mount();
    await fillAndExtract();
    await choose("Interviewer model (required)", "Linh");
    await choose("Interviewer voice (required)", "Ava");
    fireEvent.click(
      screen.getByRole("checkbox", { name: /i reviewed these requirements/i }),
    );
    const submit = screen.getByRole("button", {
      name: "Confirm & generate questions",
    });
    fireEvent.click(submit);
    fireEvent.click(submit);
    expect(
      await screen.findByRole("button", { name: "Generating questions…" }),
    ).toBeDisabled();
    reject(
      failure(
        400,
        "VALIDATION_ERROR",
        "required voice must be an active voice profile",
      ),
    );
    expect(
      await screen.findByText(
        "required voice must be an active voice profile Your draft is still here.",
      ),
    ).toBeInTheDocument();
    expect(
      mocks.post.mock.calls.filter(([path]) => path === "/job-postings"),
    ).toHaveLength(1);
    expect(screen.getByLabelText("Requirement 1")).toHaveValue("Go");
  });

  it(
    "asks for confirmation again when requirements or the JD change after ticking",
    async () => {
      mocks.post.mockImplementation((path: string) =>
        Promise.resolve(
          path === "/job-postings/documents"
            ? uploaded
            : path === "/job-postings/extract"
              ? envelope({
                  title: "Backend Engineer",
                  seniority_level: "senior",
                  requirements: [
                    {
                      name: "Go",
                      target: "programming_language",
                      type: "required",
                    },
                  ],
                })
              : envelope(created),
        ),
      );
      mount();
      await fillAndExtract();
      await choose("Interviewer model (required)", "Linh");
      await choose("Interviewer voice (required)", "Ava");
      const confirm = () =>
        screen.getByRole("checkbox", {
          name: /i reviewed these requirements/i,
        });

      // Editing a requirement row clears the tick.
      fireEvent.click(confirm());
      expect(confirm()).toBeChecked();
      fireEvent.change(screen.getByLabelText("Requirement 1"), {
        target: { value: "Python" },
      });
      await waitFor(() => expect(confirm()).not.toBeChecked());

      // Changing the seniority clears it too.
      fireEvent.click(confirm());
      await choose("Seniority (required)", "Junior");
      await waitFor(() => expect(confirm()).not.toBeChecked());

      // So does a different JD PDF.
      fireEvent.click(confirm());
      mocks.post.mockImplementation((path: string) =>
        Promise.resolve(
          path === "/job-postings/documents"
            ? envelope({
                document_id: "88888888-8888-4888-8888-888888888888",
                raw_text:
                  "We hire Python developers for data pipelines. ".repeat(3),
              })
            : envelope(created),
        ),
      );
      pickFile(pdf("python.pdf"));
      expect(await screen.findByText("python.pdf")).toBeInTheDocument();
      await waitFor(() => expect(confirm()).not.toBeChecked());

      fireEvent.click(
        screen.getByRole("button", { name: "Confirm & generate questions" }),
      );
      expect(
        await screen.findByText(
          "Confirm that you reviewed the requirements before submitting.",
        ),
      ).toBeInTheDocument();
      expect(
        mocks.post.mock.calls.filter(([path]) => path === "/job-postings"),
      ).toHaveLength(0);
    },
    LONG_FLOW_MS,
  );

  it("links to Personal Avatar Studio when the recruiter has no interviewer model", async () => {
    mocks.get.mockImplementation((path: string) =>
      Promise.resolve(
        path === "/voices"
          ? pageOf([{ id: voiceId, name: "Ava" }])
          : pageOf([]),
      ),
    );
    mount();
    expect(
      await screen.findByText(/you don't have an interviewer model yet/i),
    ).toBeInTheDocument();
    const link = screen.getByRole("link", {
      name: /create one in personal avatar studio/i,
    });
    expect(link).toHaveAttribute("href", "/avatar-studio");
    expect(link).toHaveAttribute("target", "_blank");
  });

  it("adds and removes requirements by hand", async () => {
    mount();
    fireEvent.click(screen.getByRole("button", { name: "Add requirement" }));
    fireEvent.change(await screen.findByLabelText("Requirement 1"), {
      target: { value: "Kafka" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Remove Kafka" }));
    expect(screen.queryByLabelText("Requirement 1")).not.toBeInTheDocument();
  });
});

it(
  "retries failed generation on the saved posting without creating or submitting again",
  async () => {
    let attempts = 0;
    mocks.post.mockImplementation((path: string) => {
      if (path === "/job-postings/documents") return Promise.resolve(uploaded);
      if (path === "/job-postings/extract")
        return Promise.resolve(
          envelope({
            title: "Backend Engineer",
            seniority_level: "senior",
            requirements: created.requirements,
          }),
        );
      if (path === "/job-postings")
        return Promise.resolve(
          envelope({ ...created, review_submitted: false }),
        );
      if (path === `/job-postings/${postingId}/questions/generate`) {
        attempts++;
        return attempts === 1
          ? Promise.reject(
              failure(502, "QUESTION_GENERATION_FAILED", "LLM unavailable"),
            )
          : Promise.resolve(envelope(generated));
      }
      throw new Error(`Unexpected request: ${path}`);
    });
    mount();
    await fillAndExtract();
    await choose("Interviewer model (required)", "Linh");
    await choose("Interviewer voice (required)", "Ava");
    fireEvent.click(
      screen.getByRole("checkbox", { name: /i reviewed these requirements/i }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Confirm & generate questions" }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "LLM unavailable",
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Retry generating questions" }),
    );
    await waitFor(() =>
      expect(mocks.replace).toHaveBeenCalledWith(
        `/recruiter/job-postings/${postingId}/questions`,
      ),
    );
    expect(
      mocks.post.mock.calls.filter(([path]) => path === "/job-postings"),
    ).toHaveLength(1);
    expect(
      mocks.post.mock.calls.filter(
        ([path]) => path === `/job-postings/${postingId}/questions/generate`,
      ),
    ).toHaveLength(2);
    expect(
      mocks.post.mock.calls.some(([path]) => String(path).endsWith("/submit")),
    ).toBe(false);
  },
  LONG_FLOW_MS,
);

it(
  "returns to the saved posting when it was locked meanwhile",
  async () => {
    mocks.post.mockImplementation((path: string) => {
      if (path === "/job-postings/documents") return Promise.resolve(uploaded);
      if (path === "/job-postings/extract")
        return Promise.resolve(
          envelope({
            title: "Backend Engineer",
            seniority_level: "senior",
            requirements: created.requirements,
          }),
        );
      if (path === "/job-postings") return Promise.resolve(envelope(created));
      return Promise.reject(
        failure(
          409,
          "QUESTION_BANK_LOCKED",
          "the question bank can only be changed while the job posting is pending or rejected",
        ),
      );
    });
    mount();
    await fillAndExtract();
    await choose("Interviewer model (required)", "Linh");
    await choose("Interviewer voice (required)", "Ava");
    fireEvent.click(
      screen.getByRole("checkbox", { name: /i reviewed these requirements/i }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Confirm & generate questions" }),
    );
    await waitFor(() =>
      expect(mocks.replace).toHaveBeenCalledWith(
        `/recruiter/job-postings/${postingId}`,
      ),
    );
    expect(
      mocks.post.mock.calls.filter(
        ([path]) => path === `/job-postings/${postingId}/questions/generate`,
      ),
    ).toHaveLength(1);
  },
  LONG_FLOW_MS,
);
