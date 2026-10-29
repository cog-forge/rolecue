import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApplyDialog } from "@/features/candidate/applications/components/apply-dialog";

const mocks = vi.hoisted(() => ({
  post: vi.fn(),
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

const posting = {
  id: "33333333-3333-4333-8333-333333333333",
  title: "Backend Engineer",
  company_name: "Acme",
  location: "Ha Noi",
};
const applicationId = "44444444-4444-4444-8444-444444444444";

function renderDialog(onOpenChange = vi.fn()) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <ApplyDialog posting={posting} open onOpenChange={onOpenChange} />
    </QueryClientProvider>,
  );
  return onOpenChange;
}

function pick(file: File) {
  const input = document.getElementById("cv-file");
  if (!(input instanceof HTMLInputElement)) throw new Error("CV input missing");
  fireEvent.change(input, { target: { files: [file] } });
}

beforeEach(() => {
  mocks.post.mockReset();
});

describe("ApplyDialog", () => {
  it("summarises the posting and says no interview starts yet", () => {
    renderDialog();
    expect(
      screen.getByRole("dialog", { name: "Apply for Backend Engineer" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/Acme · Ha Noi/)).toBeInTheDocument();
    expect(screen.getByText(/no interview starts/i)).toBeInTheDocument();
  });

  it("shows what happens next after sending, with a link to the new application", async () => {
    mocks.post.mockResolvedValue({
      data: {
        success: true,
        data: {
          id: applicationId,
          job_posting_id: posting.id,
          status: "pending",
          created_at: "2026-10-29T08:00:00Z",
        },
      },
    });
    const onOpenChange = renderDialog();
    pick(new File([new Uint8Array(10)], "cv.pdf", { type: "application/pdf" }));
    fireEvent.click(screen.getByRole("button", { name: "Send application" }));

    expect(
      await screen.findByRole("heading", { name: "Application sent" }),
    ).toBeInTheDocument();
    const current = screen
      .getAllByRole("listitem")
      .find((item) => item.getAttribute("aria-current") === "step");
    expect(current).toHaveTextContent("The recruiter reviews your CV");
    expect(
      screen.getByRole("link", { name: "View my application" }),
    ).toHaveAttribute("href", `/applications/${applicationId}`);
    fireEvent.click(screen.getByRole("button", { name: "Keep browsing" }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});
