import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { renderWithIntl } from "@tests/helpers/intl";
import { GradingEditor } from "@/features/recruiter/question-bank/components/grading-editor";
import { GradingBank } from "@/features/recruiter/question-bank/components/grading-bank";
import { bankSchema } from "@/features/recruiter/question-bank/schemas/question-bank-schema";
import type { Rubric } from "@/features/rubrics/schemas/rubric-schema";
const api = vi.hoisted(() => ({ get: vi.fn(), patch: vi.fn(), post: vi.fn() }));
vi.mock("@/lib/api/client", () => ({ apiClient: api }));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => ({ id: "recruiter" }),
}));
const id = "11111111-1111-4111-8111-111111111111",
  rid = "22222222-2222-4222-8222-222222222222",
  postingId = "33333333-3333-4333-8333-333333333333";
const question = {
  id,
  content: "Explain a mechanism.",
  rubric_id: rid,
  expected_answer: null,
  grading_incomplete: true,
  grading_issues: ["missing_expected_answer"],
  revision: "a".repeat(64),
};
const bank = bankSchema.parse({
  questions: [question],
  posting_status: "pending",
  editable: true,
  max_question_length: 1000,
  generation_size: 20,
  review_submitted: false,
});
const rubric: Rubric = {
  id: rid,
  name: "Mechanism rubric",
  description: "",
  question_type: "concept",
  scope: "system",
  is_active: true,
  updated_at: "2026-11-11",
  created_at: "2026-11-11",
  criteria_count: 1,
  max_points: 3,
  is_own: false,
  can_manage: false,
  locked: true,
};
const envelope = (data: unknown) => ({ data: { success: true, data } });
const failure = (code: string) =>
  new AxiosError("error", "ERR_BAD_RESPONSE", undefined, undefined, {
    status: 409,
    statusText: "Conflict",
    data: { error: { code } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
function mount(children: React.ReactNode) {
  return renderWithIntl(
    <QueryClientProvider
      client={
        new QueryClient({
          defaultOptions: {
            queries: { retry: false },
            mutations: { retry: false },
          },
        })
      }
    >
      {children}
    </QueryClientProvider>,
  );
}
const callbacks = () => ({
  onSaved: vi.fn(),
  onDirtyChange: vi.fn(),
  onPendingChange: vi.fn(),
});
beforeEach(() => vi.clearAllMocks());
describe("grading review", () => {
  it("saves an incomplete answer draft without requiring a nonexistent rubric confirmation", async () => {
    const cb = callbacks();
    mount(
      <GradingEditor
        question={{ ...bank.questions[0], rubric_id: null }}
        number={1}
        postingId={postingId}
        bank={bank}
        rubrics={[rubric]}
        {...cb}
      />,
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Review grading for question 1" }),
    );
    fireEvent.change(screen.getByLabelText("Expected answer"), {
      target: { value: "Draft key points." },
    });
    expect(
      screen.queryByRole("checkbox", { name: /I reviewed/ }),
    ).not.toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Save draft" })).toBeEnabled(),
    );
  });
  it("requires a review after bulk assignment, then saves the confirmed answer with its revision", async () => {
    const cb = callbacks();
    api.patch.mockResolvedValue(
      envelope({
        ...question,
        expected_answer: "Key points",
        revision: "b".repeat(64),
        grading_incomplete: false,
        grading_issues: [],
      }),
    );
    mount(
      <GradingEditor
        question={bank.questions[0]}
        number={1}
        postingId={postingId}
        bank={bank}
        rubrics={[rubric]}
        {...cb}
      />,
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Review grading for question 1" }),
    );
    fireEvent.change(screen.getByLabelText("Expected answer"), {
      target: { value: "Key points" },
    });
    expect(screen.getByRole("button", { name: "Save draft" })).toBeDisabled();
    fireEvent.click(screen.getByRole("checkbox", { name: /I reviewed/ }));
    expect(
      screen.queryByRole("checkbox", { name: /I reviewed/ }),
    ).toHaveAttribute("aria-checked", "true");
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Save draft" })).toBeEnabled(),
    );
    fireEvent.click(screen.getByRole("button", { name: "Save draft" }));
    await waitFor(() => expect(cb.onSaved).toHaveBeenCalledOnce());
    expect(api.patch).toHaveBeenCalledWith(
      `/job-postings/${postingId}/questions/${id}`,
      {
        rubric_id: rid,
        expected_answer: "Key points",
        previous_revision: question.revision,
      },
    );
    expect(screen.getByRole("status")).toHaveTextContent("Draft saved.");
  });
  it("retains a failed draft and requires an explicit reload and reapply after a stale edit", async () => {
    const cb = callbacks();
    api.patch.mockRejectedValueOnce(failure("QUESTION_STALE"));
    api.get.mockResolvedValue(
      envelope({
        ...bank,
        questions: [
          {
            ...question,
            expected_answer: "Changed elsewhere",
            revision: "b".repeat(64),
          },
        ],
      }),
    );
    mount(
      <GradingEditor
        question={bank.questions[0]}
        number={1}
        postingId={postingId}
        bank={bank}
        rubrics={[rubric]}
        {...cb}
      />,
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Review grading for question 1" }),
    );
    fireEvent.change(screen.getByLabelText("Expected answer"), {
      target: { value: "My draft" },
    });
    fireEvent.click(screen.getByRole("checkbox", { name: /I reviewed/ }));
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Save draft" })).toBeEnabled(),
    );
    fireEvent.click(screen.getByRole("button", { name: "Save draft" }));
    await screen.findByText("This question changed in another tab.");
    expect(screen.getByLabelText("Expected answer")).toHaveValue("My draft");
    expect(screen.getByRole("button", { name: "Save draft" })).toBeDisabled();
    fireEvent.click(
      screen.getByRole("button", { name: "Reload current question" }),
    );
    await screen.findByText("Changed elsewhere");
    fireEvent.click(
      screen.getByRole("button", {
        name: "Keep my draft against this version",
      }),
    );
    expect(screen.getByLabelText("Expected answer")).toHaveValue("My draft");
    api.patch.mockResolvedValue(
      envelope({
        ...question,
        expected_answer: "My draft",
        revision: "c".repeat(64),
        grading_incomplete: false,
        grading_issues: [],
      }),
    );
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Save draft" })).toBeEnabled(),
    );
    fireEvent.click(screen.getByRole("button", { name: "Save draft" }));
    await waitFor(() =>
      expect(api.patch).toHaveBeenLastCalledWith(
        expect.any(String),
        expect.objectContaining({ previous_revision: "b".repeat(64) }),
      ),
    );
  });
  it("exposes missing configuration before submit and keeps user content unchanged", async () => {
    api.get.mockResolvedValue(
      envelope({ items: [rubric], pagination: { total: 1 } }),
    );
    const cb = callbacks();
    mount(
      <GradingBank
        bank={bank}
        postingId={postingId}
        renderQuestion={(q) => <p>{q.content}</p>}
        disabled={false}
        {...cb}
      />,
    );
    expect(
      screen.getByText("1 question needs grading configuration"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", {
        name: "Question 1: Write an expected answer",
      }),
    ).toHaveAttribute("href", `#question-${id}`);
    expect(screen.getByText("Explain a mechanism.")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Change rubric" }),
    ).not.toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("checkbox", { name: "Select question 1" }),
    );
    expect(screen.getByRole("button", { name: "Change rubric" })).toBeEnabled();
    fireEvent.click(screen.getByRole("button", { name: "Change rubric" }));
    expect(
      screen.getByRole("dialog", {
        name: "Apply a rubric to selected questions",
      }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Apply rubric" })).toBeDisabled();
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
  });
});
