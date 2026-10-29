import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { CvApplyForm } from "@/features/candidate/applications/components/cv-apply-form";

const mocks = vi.hoisted(() => ({
  post: vi.fn(),
  applied: vi.fn(),
  busy: vi.fn(),
  user: {
    id: "11111111-1111-4111-8111-111111111111",
    full_name: "Cand",
    email: "cand@example.com",
    role: "candidate" as const,
    email_verified: true,
    is_locked: false,
    image: null,
  },
}));
vi.mock("@/lib/api/client", () => ({ apiClient: { post: mocks.post } }));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));

const postingId = "33333333-3333-4333-8333-333333333333";
const applicationId = "44444444-4444-4444-8444-444444444444";
const created = {
  success: true,
  data: {
    id: applicationId,
    job_posting_id: postingId,
    status: "pending",
    created_at: "2026-10-25T08:00:00Z",
  },
};

const pdf = (name = "cv.pdf", size = 1024) =>
  new File([new Uint8Array(size)], name, { type: "application/pdf" });

function apiFailure(status: number, code: string, message = "server says") {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: { success: false, error: { code, message } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function renderForm() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <CvApplyForm
        postingId={postingId}
        onApplied={mocks.applied}
        onBusyChange={mocks.busy}
      />
    </QueryClientProvider>,
  );
}

function pick(file: File) {
  const input = document.getElementById("cv-file");
  if (!(input instanceof HTMLInputElement)) throw new Error("CV input missing");
  fireEvent.change(input, { target: { files: [file] } });
}

const applyButton = () =>
  screen.getByRole("button", { name: "Send application" });

beforeEach(() => {
  vi.clearAllMocks();
  mocks.post.mockReset();
});

describe("CvApplyForm", () => {
  it("refuses a non-PDF and an oversized file without uploading", () => {
    renderForm();
    pick(new File(["hello"], "cv.txt", { type: "text/plain" }));
    expect(screen.getByRole("alert")).toHaveTextContent("Choose a PDF file.");
    expect(applyButton()).toBeDisabled();

    pick(pdf("big.pdf", 5 * 1024 * 1024 + 1));
    expect(screen.getByRole("alert")).toHaveTextContent(
      "That PDF is too large. Choose one up to 5 MB.",
    );
    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("states the backend limits before any upload", () => {
    renderForm();
    expect(
      screen.getByText(/one pdf with selectable text, up to 5 mb/i),
    ).toBeInTheDocument();
    expect(applyButton()).toBeDisabled();
  });

  it("accepts a dropped PDF and names it before sending", () => {
    renderForm();
    const zone = screen.getByText(/or drop it here/i).parentElement;
    if (!zone) throw new Error("drop zone missing");
    fireEvent.drop(zone, { dataTransfer: { files: [pdf("dropped.pdf")] } });
    expect(screen.getByText("dropped.pdf")).toBeInTheDocument();
    expect(applyButton()).toBeEnabled();
  });

  it("shows upload progress, then hands the new Application to the caller", async () => {
    const gate = deferred<unknown>();
    mocks.post.mockReturnValue(gate.promise);
    renderForm();
    pick(pdf());
    fireEvent.click(applyButton());

    // The mutation starts on the next tick, so wait for the request to leave.
    await waitFor(() => expect(mocks.post).toHaveBeenCalledTimes(1));
    expect(mocks.post).toHaveBeenCalledWith(
      `/job-board/${postingId}/applications`,
      expect.any(FormData),
      { onUploadProgress: expect.any(Function) },
    );
    const body = mocks.post.mock.calls[0][1] as FormData;
    expect((body.get("file") as File).name).toBe("cv.pdf");
    const config = mocks.post.mock.calls[0][2] as {
      onUploadProgress: (event: { loaded: number; total: number }) => void;
    };

    act(() => config.onUploadProgress({ loaded: 40, total: 100 }));
    expect(
      screen.getByRole("progressbar", { name: "CV upload progress" }),
    ).toHaveAttribute("aria-valuenow", "40");
    expect(screen.getByRole("status")).toHaveTextContent(
      "Uploading your CV… 40%",
    );

    expect(mocks.busy).toHaveBeenLastCalledWith(true);
    await act(async () => gate.resolve({ data: created }));
    await waitFor(() =>
      expect(mocks.applied).toHaveBeenCalledWith(
        expect.objectContaining({ id: applicationId }),
      ),
    );
    expect(mocks.busy).toHaveBeenLastCalledWith(false);
  });

  it("shows the failure and retries with the same file", async () => {
    const file = pdf();
    mocks.post
      .mockRejectedValueOnce(apiFailure(500, "INTERNAL_ERROR"))
      .mockResolvedValueOnce({ data: created });
    renderForm();
    pick(file);
    fireEvent.click(applyButton());

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "We couldn't upload your CV. Try again.",
    );
    expect(mocks.applied).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));

    await waitFor(() =>
      expect(mocks.applied).toHaveBeenCalledWith(
        expect.objectContaining({ id: applicationId }),
      ),
    );
    expect(mocks.post).toHaveBeenCalledTimes(2);
    expect((mocks.post.mock.calls[1][1] as FormData).get("file")).toBe(file);
  });

  it("blocks a second application with a clear message and no retry", async () => {
    mocks.post.mockRejectedValue(apiFailure(409, "ALREADY_APPLIED"));
    renderForm();
    pick(pdf());
    fireEvent.click(applyButton());

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "You already applied to this posting.",
    );
    expect(
      screen.queryByRole("button", { name: "Try again" }),
    ).not.toBeInTheDocument();
    expect(applyButton()).toBeDisabled();
    expect(
      screen.getByRole("link", { name: "View my applications" }),
    ).toHaveAttribute("href", "/applications");
    expect(mocks.applied).not.toHaveBeenCalled();
  });

  it("explains that a closed posting no longer accepts applications", async () => {
    mocks.post.mockRejectedValue(apiFailure(409, "INTAKE_NOT_OPEN"));
    renderForm();
    pick(pdf());
    fireEvent.click(applyButton());

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "This posting is no longer accepting applications.",
    );
    expect(
      screen.queryByRole("button", { name: "Try again" }),
    ).not.toBeInTheDocument();
    expect(mocks.applied).not.toHaveBeenCalled();
  });

  it("shows the server's own message for a validation error", async () => {
    mocks.post.mockRejectedValue(
      apiFailure(400, "VALIDATION_ERROR", "The CV is not a readable PDF."),
    );
    renderForm();
    pick(pdf());
    fireEvent.click(applyButton());

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "The CV is not a readable PDF.",
    );
    expect(
      screen.getByRole("button", { name: "Try again" }),
    ).toBeInTheDocument();
  });
});
