import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { EvaluationCriteria } from "@/features/recruiter/evaluation/components/evaluation-criteria";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  put: vi.fn(),
  delete: vi.fn(),
  push: vi.fn(),
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
  apiClient: { get: mocks.get, put: mocks.put, delete: mocks.delete },
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mocks.push, replace: mocks.replace }),
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));
vi.mock("goey-toast", () => ({ gooeyToast: { success: mocks.toast } }));

const postingId = "33333333-3333-4333-8333-333333333333";
const accuracy = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const depth = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const clarity = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const catalog = [
  { id: accuracy, name: "Technical accuracy" },
  { id: depth, name: "Depth of knowledge" },
  { id: clarity, name: "Clarity" },
];
const envelope = <T,>(data: T) => ({ data: { success: true, data } });
const weights = (over: Record<string, unknown> = {}) => ({
  configured: false,
  using_defaults: true,
  editable: true,
  posting_status: "pending",
  total: 100,
  weights: catalog.map((m) => ({
    metric_id: m.id,
    name: m.name,
    percentage: 100 / 3,
  })),
  ...over,
});
const custom = (over: Record<string, unknown> = {}) =>
  weights({
    configured: true,
    using_defaults: false,
    weights: [
      { metric_id: accuracy, name: "Technical accuracy", percentage: 70 },
      { metric_id: depth, name: "Depth of knowledge", percentage: 30 },
    ],
    ...over,
  });
function failure(status: number, code = "INTERNAL_ERROR", message = "boom") {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: { success: false, error: { code, message } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}
function serve(
  weightsResult: unknown,
  metricsResult: unknown = envelope({ items: catalog }),
) {
  mocks.get.mockImplementation((path: string) => {
    const result =
      path === "/evaluation-metrics" ? metricsResult : weightsResult;
    return result instanceof Error
      ? Promise.reject(result)
      : Promise.resolve(result);
  });
}
function mount(extra?: ReactNode) {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity },
      mutations: { retry: false },
    },
  });
  client.setQueryData(["auth", "me"], mocks.user);
  render(
    <QueryClientProvider client={client}>
      {extra}
      <EvaluationCriteria postingId={postingId} />
    </QueryClientProvider>,
  );
  return client;
}
const field = (name: string) => screen.getByLabelText(name) as HTMLInputElement;
const type = (name: string, value: string) =>
  fireEvent.change(field(name), { target: { value } });

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
  mocks.put.mockReset();
  mocks.delete.mockReset();
});
afterEach(async () => {
  cleanup();
  // An unmounted leave guard drops its history entry with an asynchronous Back; let it land.
  await waitFor(() =>
    expect(
      (window.history.state as Record<string, unknown> | null)?.[
        "__rolecueUnsavedChanges"
      ],
    ).toBeFalsy(),
  );
});

describe("EvaluationCriteria", () => {
  it("shows the metrics from the API with the equal-split default and says it is not a blocker", async () => {
    serve(envelope(weights()));
    mount();
    expect(screen.getByRole("status")).toHaveTextContent(
      /loading the evaluation criteria/i,
    );
    expect(await screen.findByLabelText("Technical accuracy")).toBeVisible();
    expect(screen.getByLabelText("Depth of knowledge")).toBeVisible();
    expect(screen.getByLabelText("Clarity")).toBeVisible();
    expect(screen.getByText("Using the default weights")).toBeInTheDocument();
    expect(screen.getByText(/doesn’t block approval/)).toBeInTheDocument();
    expect(Number(field("Clarity").value)).toBeCloseTo(100 / 3, 10);
    // Defaults add up, but nothing is changed yet, so there is nothing to save.
    expect(
      screen.getByRole("button", { name: "Save criteria" }),
    ).toBeDisabled();
    expect(mocks.get).toHaveBeenCalledWith(
      `/job-postings/${postingId}/evaluation-weights`,
      { signal: expect.any(AbortSignal) },
    );
    expect(mocks.get).toHaveBeenCalledWith("/evaluation-metrics", {
      signal: expect.any(AbortSignal),
    });
  });

  it("shows a metric the saved set left out as empty", async () => {
    serve(envelope(custom()));
    mount();
    await screen.findByLabelText("Clarity");
    expect(field("Technical accuracy").value).toBe("70");
    expect(field("Depth of knowledge").value).toBe("30");
    expect(field("Clarity").value).toBe("");
    expect(screen.queryByText("Using the default weights")).toBeNull();
  });

  it("keeps Save off until the weights add up, then sends only metrics above 0%", async () => {
    serve(envelope(custom()));
    mocks.put.mockResolvedValue(
      envelope(
        custom({
          weights: [
            { metric_id: accuracy, name: "Technical accuracy", percentage: 50 },
            { metric_id: depth, name: "Depth of knowledge", percentage: 30 },
            { metric_id: clarity, name: "Clarity", percentage: 20 },
          ],
        }),
      ),
    );
    mount();
    await screen.findByLabelText("Clarity");
    const save = screen.getByRole("button", { name: "Save criteria" });
    expect(save).toBeDisabled();

    type("Technical accuracy", "50");
    await waitFor(() =>
      expect(
        screen.getByText(
          "The weights add up to 80, but they must add up to 100.",
        ),
      ).toBeVisible(),
    );
    expect(save).toBeDisabled();
    expect(screen.getByText("80 of 100")).toBeInTheDocument();

    type("Clarity", "20");
    await waitFor(() => expect(save).toBeEnabled());
    fireEvent.click(save);
    await waitFor(() => expect(mocks.put).toHaveBeenCalledTimes(1));
    expect(mocks.put).toHaveBeenCalledWith(
      `/job-postings/${postingId}/evaluation-weights`,
      {
        weights: [
          { metric_id: accuracy, percentage: 50 },
          { metric_id: depth, percentage: 30 },
          { metric_id: clarity, percentage: 20 },
        ],
      },
    );
    await waitFor(() =>
      expect(mocks.toast).toHaveBeenCalledWith("Evaluation criteria saved"),
    );
    // The form now holds the saved values, so nothing is left to save.
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Save criteria" }),
      ).toBeDisabled(),
    );
  });

  it("leaves a metric at 0% out of the request", async () => {
    serve(envelope(custom()));
    mocks.put.mockResolvedValue(envelope(custom()));
    mount();
    await screen.findByLabelText("Clarity");
    type("Technical accuracy", "100");
    type("Depth of knowledge", "0");
    const save = screen.getByRole("button", { name: "Save criteria" });
    await waitFor(() => expect(save).toBeEnabled());
    fireEvent.click(save);
    await waitFor(() => expect(mocks.put).toHaveBeenCalled());
    expect(mocks.put.mock.calls[0][1]).toEqual({
      weights: [{ metric_id: accuracy, percentage: 100 }],
    });
  });

  it("shows a message next to a field that cannot be sent", async () => {
    serve(envelope(custom()));
    mount();
    await screen.findByLabelText("Clarity");
    type("Clarity", "-4");
    const message = await screen.findByText("A weight can't be negative.");
    expect(message).toHaveAttribute("role", "alert");
    expect(field("Clarity")).toHaveAttribute("aria-invalid", "true");
    expect(field("Clarity").getAttribute("aria-describedby")).toBe(message.id);
    expect(
      screen.getByRole("button", { name: "Save criteria" }),
    ).toBeDisabled();
    expect(mocks.put).not.toHaveBeenCalled();
  });

  it("keeps the draft and shows the server's message when saving fails", async () => {
    serve(envelope(custom()));
    mocks.put.mockRejectedValue(
      failure(
        400,
        "EVALUATION_WEIGHT_TOTAL_MISMATCH",
        "the weights must add up to 100",
      ),
    );
    mount();
    await screen.findByLabelText("Clarity");
    type("Technical accuracy", "50");
    type("Clarity", "20");
    const save = screen.getByRole("button", { name: "Save criteria" });
    await waitFor(() => expect(save).toBeEnabled());
    fireEvent.click(save);
    expect(
      await screen.findByText(
        "the weights must add up to 100 Your changes are still here.",
      ),
    ).toBeInTheDocument();
    expect(field("Technical accuracy").value).toBe("50");
    expect(field("Clarity").value).toBe("20");
    expect(mocks.toast).not.toHaveBeenCalled();
  });

  it("refreshes into the read-only view when the posting got approved meanwhile", async () => {
    serve(envelope(custom()));
    mocks.put.mockRejectedValue(
      failure(409, "EVALUATION_WEIGHTS_LOCKED", "locked"),
    );
    mount();
    await screen.findByLabelText("Clarity");
    type("Technical accuracy", "50");
    type("Clarity", "20");
    const save = screen.getByRole("button", { name: "Save criteria" });
    await waitFor(() => expect(save).toBeEnabled());
    serve(envelope(custom({ editable: false, posting_status: "open" })));
    fireEvent.click(save);
    // The notice is the stale-specific one, and it survives the form being replaced.
    expect(
      await screen.findByText(
        "This posting changed, so we refreshed the criteria and your unsaved changes were discarded.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/so its evaluation criteria can no longer be edited/),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.queryByLabelText("Clarity")).not.toBeInTheDocument(),
    );
    expect(screen.queryByRole("button", { name: "Save criteria" })).toBeNull();
  });

  it("is read-only once the posting is open", async () => {
    serve(envelope(custom({ editable: false, posting_status: "open" })));
    mount();
    expect(
      await screen.findByText(
        /so its evaluation criteria can no longer be edited/,
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Technical accuracy")).toBeInTheDocument();
    expect(screen.getByText("70%")).toBeInTheDocument();
    expect(screen.getByText("30%")).toBeInTheDocument();
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(screen.queryByRole("spinbutton")).toBeNull();
    expect(screen.queryByRole("button", { name: "Save criteria" })).toBeNull();
  });

  it("warns before leaving with unsaved weights, and only then", async () => {
    serve(envelope(custom()));
    mount();
    await screen.findByLabelText("Clarity");
    const clean = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(clean);
    expect(clean.defaultPrevented).toBe(false);

    type("Clarity", "5");
    await waitFor(() => {
      const dirty = new Event("beforeunload", { cancelable: true });
      window.dispatchEvent(dirty);
      expect(dirty.defaultPrevented).toBe(true);
    });
    fireEvent.click(screen.getByRole("button", { name: "Discard changes" }));
    await waitFor(() => expect(field("Clarity").value).toBe(""));
    const after = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(after);
    expect(after.defaultPrevented).toBe(false);
  });

  it("keeps the notice and says the weights were discarded when the posting is still editable", async () => {
    serve(envelope(custom()));
    mocks.put.mockRejectedValue(
      failure(409, "EVALUATION_WEIGHTS_LOCKED", "locked"),
    );
    mount();
    await screen.findByLabelText("Clarity");
    type("Technical accuracy", "50");
    type("Clarity", "20");
    const save = screen.getByRole("button", { name: "Save criteria" });
    await waitFor(() => expect(save).toBeEnabled());
    fireEvent.click(save);
    expect(
      await screen.findByText(/your unsaved changes were discarded/),
    ).toBeInTheDocument();
    // The form is rebuilt on the server's values.
    await waitFor(() => expect(field("Technical accuracy").value).toBe("70"));
    expect(field("Clarity").value).toBe("");
  });

  it("asks before leaving through any in-app link with unsaved weights", async () => {
    serve(envelope(custom()));
    // Stands in for the workspace sidebar, which lives outside the editor.
    mount(<a href="/recruiter/applications">Applications</a>);
    await screen.findByLabelText("Clarity");
    const back = screen.getByRole("link", { name: /Back to the job posting/ });
    const sidebar = screen.getByRole("link", { name: "Applications" });
    // Keeps jsdom from trying to load a link the guard lets through.
    const stopLoad = (event: Event) => event.preventDefault();
    document.addEventListener("click", stopLoad);

    // Nothing changed yet: no question.
    fireEvent.click(back);
    expect(screen.queryByRole("dialog")).toBeNull();

    type("Clarity", "5");
    await waitFor(() => expect(field("Clarity").value).toBe("5"));
    fireEvent.click(sidebar);
    const dialog = await screen.findByRole("dialog", {
      name: "Leave without saving?",
    });
    // Staying keeps the page and the draft.
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Stay on this page" }),
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(field("Clarity").value).toBe("5");
    expect(mocks.push).not.toHaveBeenCalled();
    expect(mocks.replace).not.toHaveBeenCalled();

    // Leaving goes where the link pointed, in place of the guard's extra history entry.
    fireEvent.click(back);
    fireEvent.click(
      within(
        await screen.findByRole("dialog", { name: "Leave without saving?" }),
      ).getByRole("button", { name: "Leave without saving" }),
    );
    expect(mocks.replace).toHaveBeenCalledWith(
      `/recruiter/job-postings/${postingId}`,
    );
    document.removeEventListener("click", stopLoad);
  });

  it("asks before the browser's Back button leaves unsaved weights", async () => {
    serve(envelope(custom()));
    mount();
    await screen.findByLabelText("Clarity");
    type("Clarity", "5");
    await waitFor(() => expect(field("Clarity").value).toBe("5"));
    window.history.back();
    const dialog = await screen.findByRole("dialog", {
      name: "Leave without saving?",
    });
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Stay on this page" }),
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(field("Clarity").value).toBe("5");
  });

  it("does not ask after the weights were saved", async () => {
    serve(envelope(custom()));
    mocks.put.mockResolvedValue(envelope(custom()));
    mount(<a href="/recruiter/applications">Applications</a>);
    await screen.findByLabelText("Clarity");
    type("Technical accuracy", "50");
    type("Clarity", "20");
    const save = screen.getByRole("button", { name: "Save criteria" });
    await waitFor(() => expect(save).toBeEnabled());
    fireEvent.click(save);
    await waitFor(() => expect(mocks.toast).toHaveBeenCalled());
    await waitFor(() => expect(field("Clarity").value).toBe(""));
    const stopLoad = (event: Event) => event.preventDefault();
    document.addEventListener("click", stopLoad);
    fireEvent.click(screen.getByRole("link", { name: "Applications" }));
    document.removeEventListener("click", stopLoad);
    expect(screen.queryByRole("dialog")).toBeNull();
    const unload = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(unload);
    expect(unload.defaultPrevented).toBe(false);
  });

  it("reloads the weights and the metrics catalog when a metric no longer exists", async () => {
    serve(envelope(custom()));
    mocks.put.mockRejectedValue(
      failure(400, "EVALUATION_METRIC_UNKNOWN", "unknown evaluation metric"),
    );
    mount();
    await screen.findByLabelText("Clarity");
    type("Technical accuracy", "50");
    type("Clarity", "20");
    const save = screen.getByRole("button", { name: "Save criteria" });
    await waitFor(() => expect(save).toBeEnabled());
    // Clarity was removed from the catalog meanwhile.
    serve(
      envelope(custom()),
      envelope({ items: catalog.filter((m) => m.id !== clarity) }),
    );
    mocks.get.mockClear();
    fireEvent.click(save);
    expect(
      await screen.findByText(/your unsaved changes were discarded/),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.queryByLabelText("Clarity")).not.toBeInTheDocument(),
    );
    expect(field("Technical accuracy").value).toBe("70");
    expect(mocks.get).toHaveBeenCalledWith("/evaluation-metrics", {
      signal: expect.any(AbortSignal),
    });
    expect(mocks.get).toHaveBeenCalledWith(
      `/job-postings/${postingId}/evaluation-weights`,
      { signal: expect.any(AbortSignal) },
    );
    // The stale error is answered by the refresh, not shown as a save failure.
    expect(screen.queryByText(/unknown evaluation metric/)).toBeNull();
  });

  it("keeps an error the API does not tie to a metric at form level", async () => {
    serve(envelope(custom()));
    mocks.put.mockRejectedValue(
      failure(400, "EVALUATION_WEIGHT_INVALID", "a weight must be at most 100"),
    );
    mount();
    await screen.findByLabelText("Clarity");
    type("Technical accuracy", "50");
    type("Clarity", "20");
    const save = screen.getByRole("button", { name: "Save criteria" });
    await waitFor(() => expect(save).toBeEnabled());
    fireEvent.click(save);
    expect(
      await screen.findByText(
        "a weight must be at most 100 Your changes are still here.",
      ),
    ).toHaveAttribute("role", "alert");
    expect(field("Technical accuracy")).not.toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(field("Clarity").value).toBe("20");
  });

  it("resets saved weights to the equal-split defaults after confirming", async () => {
    serve(envelope(custom()));
    mocks.delete.mockResolvedValue(envelope(weights()));
    mount();
    await screen.findByLabelText("Clarity");
    fireEvent.click(
      screen.getByRole("button", { name: "Reset to default weights" }),
    );
    const dialog = await screen.findByRole("dialog", {
      name: "Reset to the default weights?",
    });
    expect(dialog).toHaveTextContent(
      "the total of 100 split equally across every metric",
    );
    // Cancelling deletes nothing.
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Keep my weights" }),
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(mocks.delete).not.toHaveBeenCalled();

    fireEvent.click(
      screen.getByRole("button", { name: "Reset to default weights" }),
    );
    serve(envelope(weights()));
    mocks.get.mockClear();
    fireEvent.click(
      within(
        await screen.findByRole("dialog", {
          name: "Reset to the default weights?",
        }),
      ).getByRole("button", { name: "Reset weights" }),
    );
    await waitFor(() =>
      expect(mocks.delete).toHaveBeenCalledWith(
        `/job-postings/${postingId}/evaluation-weights`,
      ),
    );
    expect(
      await screen.findByText("Using the default weights"),
    ).toBeInTheDocument();
    expect(mocks.toast).toHaveBeenCalledWith("Default weights restored");
    expect(Number(field("Clarity").value)).toBeCloseTo(100 / 3, 10);
    expect(
      screen.queryByRole("button", { name: "Reset to default weights" }),
    ).toBeNull();
    // The view is refetched, not only taken from the reset response.
    expect(mocks.get).toHaveBeenCalledWith(
      `/job-postings/${postingId}/evaluation-weights`,
      { signal: expect.any(AbortSignal) },
    );
  });

  it("offers no reset while the defaults are in use", async () => {
    serve(envelope(weights()));
    mount();
    await screen.findByLabelText("Clarity");
    expect(
      screen.queryByRole("button", { name: "Reset to default weights" }),
    ).toBeNull();
  });

  it("refreshes into the read-only view when a reset finds the posting locked", async () => {
    serve(envelope(custom()));
    mocks.delete.mockRejectedValue(
      failure(409, "EVALUATION_WEIGHTS_LOCKED", "locked"),
    );
    mount();
    await screen.findByLabelText("Clarity");
    fireEvent.click(
      screen.getByRole("button", { name: "Reset to default weights" }),
    );
    serve(envelope(custom({ editable: false, posting_status: "open" })));
    fireEvent.click(
      within(
        await screen.findByRole("dialog", {
          name: "Reset to the default weights?",
        }),
      ).getByRole("button", { name: "Reset weights" }),
    );
    expect(
      await screen.findByText(
        /so its evaluation criteria can no longer be edited/,
      ),
    ).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("keeps the reset dialog open with the server's message when a reset fails", async () => {
    serve(envelope(custom()));
    mocks.delete.mockRejectedValue(failure(500, "INTERNAL_ERROR", "boom"));
    mount();
    await screen.findByLabelText("Clarity");
    fireEvent.click(
      screen.getByRole("button", { name: "Reset to default weights" }),
    );
    const dialog = await screen.findByRole("dialog", {
      name: "Reset to the default weights?",
    });
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Reset weights" }),
    );
    expect(await within(dialog).findByRole("alert")).toHaveTextContent("boom");
    expect(field("Technical accuracy").value).toBe("70");
  });

  it("words the default notice from the total", async () => {
    serve(envelope(weights()));
    mount();
    expect(
      await screen.findByText(
        /the total of 100 split equally across every metric/,
      ),
    ).toBeInTheDocument();
  });

  it("does not claim a split of a total when there is no sum rule", async () => {
    serve(envelope(weights({ total: 0 })));
    mount();
    expect(
      await screen.findByText(/every metric counting the same/),
    ).toBeInTheDocument();
    expect(screen.queryByText(/split equally/)).toBeNull();
  });

  it("shows an error with a retry", async () => {
    serve(failure(500));
    mount();
    expect(
      await screen.findByText("We couldn't load the evaluation criteria"),
    ).toBeInTheDocument();
    serve(envelope(custom()));
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByLabelText("Clarity")).toBeVisible();
  });

  it("treats a posting that is not the Recruiter's as not found", async () => {
    serve(failure(404));
    mount();
    expect(
      await screen.findByText("We couldn't find this job posting"),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Try again" })).toBeNull();
  });

  it("re-checks the session when access is denied", async () => {
    serve(failure(403));
    const client = mount();
    expect(
      await screen.findByText("You don't have access to evaluation criteria"),
    ).toBeInTheDocument();
    expect(client.getQueryState(["auth", "me"])?.isInvalidated).toBe(true);
  });
});
