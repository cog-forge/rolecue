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
  core_question_count: 0,
  created_at: "2026-10-12T08:00:00Z",
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
async function fillAndExtract() {
  fireEvent.change(screen.getByLabelText("Job description text (required)"), {
    target: { value: jdText },
  });
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
  it("extracts, requires confirmation, then submits exactly the Go body", async () => {
    mocks.post.mockImplementation((path: string) =>
      Promise.resolve(
        path === "/job-postings/extract"
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
    const client = mount();
    expect(
      screen.getByRole("button", { name: "Suggest requirements" }),
    ).toBeDisabled();
    await fillAndExtract();
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
      screen.getByRole("button", { name: "Submit for approval" }),
    );
    expect(
      await screen.findByText(
        "Confirm that you reviewed the requirements before submitting.",
      ),
    ).toBeInTheDocument();
    expect(mocks.post).toHaveBeenCalledTimes(1);

    fireEvent.click(
      screen.getByRole("checkbox", { name: /i reviewed these requirements/i }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Submit for approval" }),
    );
    await waitFor(() =>
      expect(mocks.post).toHaveBeenCalledWith("/job-postings", {
        title: "Backend Engineer",
        raw_text: jdText,
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
        `/recruiter/job-postings/${postingId}`,
      ),
    );
    expect(
      client.getQueryData([
        "recruiter-job-postings",
        mocks.user.id,
        "detail",
        postingId,
      ]),
    ).toEqual(created);
  });

  it("keeps the text and explains when extraction fails", async () => {
    mocks.post.mockRejectedValue(
      failure(502, "EXTRACTION_FAILED", "Job description extraction failed."),
    );
    mount();
    fireEvent.change(screen.getByLabelText("Job description text (required)"), {
      target: { value: jdText },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Suggest requirements" }),
    );
    expect(
      await screen.findByText(
        /job description extraction failed\. your text is still here/i,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByLabelText("Job description text (required)"),
    ).toHaveValue(jdText);
    expect(
      screen.getByRole("button", { name: "Add requirement" }),
    ).toBeEnabled();
  });

  it("shows field errors before calling the API", async () => {
    mount();
    fireEvent.click(
      screen.getByRole("button", { name: "Submit for approval" }),
    );
    expect(
      await screen.findByText(
        "Paste at least 100 characters of the job description.",
      ),
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
      path === "/job-postings/extract"
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
    const submit = screen.getByRole("button", { name: "Submit for approval" });
    fireEvent.click(submit);
    fireEvent.click(submit);
    expect(
      await screen.findByRole("button", { name: "Submitting…" }),
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

  it("says so when the recruiter has no interviewer model", async () => {
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
