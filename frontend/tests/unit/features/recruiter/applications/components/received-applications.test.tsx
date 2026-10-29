import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ReceivedApplications } from "@/features/recruiter/applications/components/received-applications";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  toast: vi.fn(),
  search: new URLSearchParams(),
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
vi.mock("goey-toast", () => ({ gooeyToast: { success: mocks.toast } }));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  useSearchParams: () => mocks.search,
  usePathname: () => "/recruiter/applications",
}));

const postingId = "33333333-3333-4333-8333-333333333333";
const id = (i: number) =>
  `44444444-4444-4444-8444-${String(i).padStart(12, "0")}`;
const application = (i: number, over: Record<string, unknown> = {}) => ({
  id: id(i),
  job_posting_id: postingId,
  job_posting_title: "Backend Engineer",
  status: "pending",
  submitted_at: "2026-10-20T08:00:00Z",
  candidate: {
    id: `55555555-5555-4555-8555-${String(i).padStart(12, "0")}`,
    name: `Candidate ${i}`,
    email: `candidate${i}@example.com`,
    image: null,
  },
  interview_eligible: false,
  interview_deadline: null,
  reject_reason: null,
  updated_at: "2026-10-20T08:00:00Z",
  ...over,
});
const detail = (i: number, over: Record<string, unknown> = {}) => ({
  ...application(i, over),
  job_posting: {
    id: postingId,
    title: "Backend Engineer",
    status: "open",
    interview_slot: 5,
    cv_approvals: 1,
  },
  cv: {
    url: `https://cv.example.test/${i}.pdf`,
    expires_at: "2099-01-01T00:00:00Z",
  },
  interview: null,
  final_review: null,
});
const envelope = (data: unknown) => ({ data: { success: true, data } });
const listPage = (items: ReturnType<typeof application>[]) =>
  envelope({
    items,
    pagination: { limit: 20, offset: 0, total: items.length },
  });

function serve(items: ReturnType<typeof application>[]) {
  mocks.get.mockImplementation((url: string) => {
    if (url === "/recruiter/applications")
      return Promise.resolve(listPage(items));
    if (url === "/job-postings")
      return Promise.resolve(
        envelope({
          items: [],
          pagination: { limit: 100, offset: 0, total: 0 },
        }),
      );
    const match = url.match(/\/applications\/([0-9a-f-]+)$/);
    const item = items.find((candidate) => candidate.id === match?.[1]);
    if (!item) return Promise.reject(new Error(`unexpected ${url}`));
    const { job_posting, cv, interview, final_review } = detail(
      Number(item.id.slice(-12)),
    );
    return Promise.resolve(
      envelope({ ...item, job_posting, cv, interview, final_review }),
    );
  });
}

function mount(selected?: string) {
  mocks.search = new URLSearchParams(selected ? { application: selected } : {});
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity },
      mutations: { retry: false },
    },
  });
  client.setQueryData(["auth", "me"], mocks.user);
  render(
    <QueryClientProvider client={client}>
      <ReceivedApplications />
    </QueryClientProvider>,
  );
  return client;
}

let push: ReturnType<typeof vi.spyOn>;
beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
  mocks.post.mockReset();
  // The review panel sits beside the list only on wide screens.
  vi.stubGlobal("matchMedia", (query: string) => ({
    matches: true,
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }));
  push = vi.spyOn(window.history, "pushState").mockImplementation(() => {});
});
afterEach(() => {
  push.mockRestore();
  vi.unstubAllGlobals();
});

describe("ReceivedApplications review inbox", () => {
  it("opens the first CV awaiting review beside the list, with the CV embedded", async () => {
    serve([
      application(1, { status: "rejected" }),
      application(2),
      application(3),
    ]);
    mount();
    const panel = await screen.findByRole("region", { name: "Candidate 2" });
    expect(within(panel).getByTitle("CV of Candidate 2")).toHaveAttribute(
      "src",
      "https://cv.example.test/2.pdf",
    );
    expect(within(panel).getByText("2 of 3")).toBeInTheDocument();
    expect(within(panel).getByText("1 of 5 approved")).toBeInTheDocument();
    expect(
      within(panel).getByRole("button", { name: "Approve CV" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("link", { name: "Review CV of Candidate 2" }),
    ).toHaveAttribute("aria-current", "true");
  });

  it("selects another row in place and keeps the filters in the URL", async () => {
    serve([application(1), application(2)]);
    mount();
    await screen.findByRole("region", { name: "Candidate 1" });
    fireEvent.click(
      screen.getByRole("link", { name: "Review CV of Candidate 2" }),
    );
    expect(push).toHaveBeenCalledWith(
      null,
      "",
      `/recruiter/applications?application=${id(2)}`,
    );
  });

  it("opens the Application named in the URL", async () => {
    serve([application(1), application(2)]);
    mount(id(2));
    expect(
      await screen.findByRole("region", { name: "Candidate 2" }),
    ).toBeInTheDocument();
  });

  it("moves to the next CV awaiting review after a decision", async () => {
    serve([
      application(1),
      application(2, { status: "rejected" }),
      application(3),
    ]);
    mocks.post.mockResolvedValue(
      envelope({
        id: id(1),
        job_posting_id: postingId,
        status: "interview_eligible",
        interview_deadline: "2099-01-01T08:00:00Z",
        reject_reason: null,
        updated_at: "2026-10-24T09:00:00Z",
        job_posting_status: "open",
      }),
    );
    mount();
    const panel = await screen.findByRole("region", { name: "Candidate 1" });
    fireEvent.click(within(panel).getByRole("button", { name: "Approve CV" }));
    await waitFor(() =>
      expect(push).toHaveBeenCalledWith(
        null,
        "",
        `/recruiter/applications?application=${id(3)}`,
      ),
    );
  });

  it("says when nothing else on the page is waiting for review", async () => {
    serve([application(1), application(2, { status: "approved" })]);
    mocks.post.mockResolvedValue(
      envelope({
        id: id(1),
        job_posting_id: postingId,
        status: "rejected",
        interview_deadline: null,
        reject_reason: null,
        updated_at: "2026-10-24T09:00:00Z",
        job_posting_status: "open",
      }),
    );
    mount();
    const panel = await screen.findByRole("region", { name: "Candidate 1" });
    fireEvent.click(within(panel).getByRole("button", { name: "Reject CV" }));
    fireEvent.click(
      within(
        await screen.findByRole("dialog", { name: "Reject this CV?" }),
      ).getByRole("button", { name: "Reject CV" }),
    );
    expect(await screen.findByText("You're all caught up")).toBeInTheDocument();
  });
});
