import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { ApplicationList } from "@/features/recruiter/applications/components/application-list";
import { ReceivedApplications } from "@/features/recruiter/applications/components/received-applications";
import {
  applicationSearch,
  defaultApplicationFilters,
  type ApplicationFilters,
} from "@/features/recruiter/applications/utils/application-filters";

// Radix Select calls scrollIntoView when it opens; jsdom does not implement it.
if (!HTMLElement.prototype.scrollIntoView)
  Object.defineProperty(HTMLElement.prototype, "scrollIntoView", {
    configurable: true,
    value: () => {},
  });

// Deadlines show in the viewer's time zone; pin one (before the component is imported) so the
// rendered text is the same on every machine.
vi.hoisted(() => {
  process.env.TZ = "Asia/Ho_Chi_Minh";
});

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  push: vi.fn(),
  search: new URLSearchParams(),
  pathname: "/recruiter/applications",
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
vi.mock("@/lib/api/client", () => ({ apiClient: { get: mocks.get } }));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mocks.push }),
  useSearchParams: () => mocks.search,
  usePathname: () => mocks.pathname,
}));

const postingId = "33333333-3333-4333-8333-333333333333";
const application = (i: number, over: Record<string, unknown> = {}) => ({
  id: `44444444-4444-4444-8444-${String(i).padStart(12, "0")}`,
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
const page = (
  items: ReturnType<typeof application>[],
  total = items.length,
  offset = 0,
) => ({
  data: {
    success: true,
    data: { items, pagination: { limit: 20, offset, total } },
  },
});
function failure(status: number) {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: {
      success: false,
      error: { code: "INTERNAL_ERROR", message: "boom" },
    },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}
// The inbox also loads the posting choices; tests only care about the Applications request.
const postingOptions = {
  data: {
    success: true,
    data: {
      items: [],
      pagination: { limit: 100, offset: 0, total: 0 },
    },
  },
};
function serve(response: unknown) {
  mocks.get.mockImplementation((url: string) => {
    if (url === "/job-postings") return Promise.resolve(postingOptions);
    return response instanceof Error
      ? Promise.reject(response)
      : Promise.resolve(response);
  });
}
const applicationsCall = (url: string) =>
  mocks.get.mock.calls.find(([called]) => called === url);

function mount(
  props: { postingId?: string } = {},
  filters: ApplicationFilters = defaultApplicationFilters,
) {
  mocks.search = new URLSearchParams(applicationSearch(filters));
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  client.setQueryData(["auth", "me"], mocks.user);
  render(
    <QueryClientProvider client={client}>
      {props.postingId ? (
        <ApplicationList postingId={props.postingId} />
      ) : (
        <ReceivedApplications />
      )}
    </QueryClientProvider>,
  );
  return client;
}
const rows = async () => screen.findByRole("list", { name: /^Applications,/ });

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
  mocks.pathname = "/recruiter/applications";
});

describe("ApplicationList", () => {
  it("lists one posting's applications with status, deadline and reject reason", async () => {
    mocks.pathname = `/recruiter/job-postings/${postingId}`;
    serve(
      page([
        application(1),
        application(2, {
          status: "interview_eligible",
          interview_eligible: true,
          interview_deadline: "2026-10-21T08:30:00Z",
        }),
        application(3, {
          status: "interview_eligible",
          interview_eligible: false,
          interview_deadline: "2026-10-19T08:30:00Z",
        }),
        application(4, { status: "rejected", reject_reason: "Not a fit" }),
      ]),
    );
    mount({ postingId });
    expect(screen.getByRole("status")).toHaveTextContent(
      /loading applications/i,
    );
    const list = await rows();
    expect(within(list).getByText("Candidate 1")).toBeInTheDocument();
    expect(
      within(list).getByText("candidate1@example.com"),
    ).toBeInTheDocument();
    expect(within(list).getByText("Awaiting CV review")).toBeInTheDocument();
    const due = within(list).getByText(/Complete interview by/);
    expect(due).toHaveTextContent(
      "Complete interview by Oct 21, 2026, 3:30 PM GMT+7",
    );
    expect(due.querySelector("time")).toHaveAttribute(
      "datetime",
      "2026-10-21T08:30:00Z",
    );
    expect(within(list).getByText(/Deadline passed/)).toHaveTextContent(
      "Deadline passed Oct 19, 2026, 3:30 PM GMT+7",
    );
    expect(within(list).getByText("Rejected")).toBeInTheDocument();
    expect(within(list).getByText("Reason: Not a fit")).toBeInTheDocument();
    // The posting is already known, so its title is left out of the rows.
    expect(within(list).queryByText(/Backend Engineer ·/)).toBeNull();
    // A pending row says what to do next; others open the Application.
    expect(
      within(list).getByRole("link", { name: "Review CV of Candidate 1" }),
    ).toHaveAttribute(
      "href",
      `/recruiter/job-postings/${postingId}/applications/${application(1).id}`,
    );
    expect(
      within(list).getByRole("link", {
        name: "Open application of Candidate 4",
      }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("combobox", { name: "Job posting" }),
    ).not.toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledWith(
      `/job-postings/${postingId}/applications`,
      { params: { limit: 20, offset: 0 }, signal: expect.any(AbortSignal) },
    );
    expect(screen.getByText("Showing 1–4 of 4")).toBeInTheDocument();
  });

  it("on a wide screen, opens the review inbox for this posting with the row selected", async () => {
    mocks.pathname = `/recruiter/job-postings/${postingId}`;
    vi.stubGlobal("matchMedia", (query: string) => ({
      matches: true,
      media: query,
      addEventListener: () => {},
      removeEventListener: () => {},
    }));
    serve(page([application(1)]));
    mount({ postingId });
    fireEvent.click(
      within(await rows()).getByRole("link", {
        name: "Review CV of Candidate 1",
      }),
    );
    expect(mocks.push).toHaveBeenCalledWith(
      `/recruiter/applications?posting=${postingId}&application=${application(1).id}`,
    );
    vi.unstubAllGlobals();
  });

  it("lists every posting's applications with the posting named in each row", async () => {
    serve(page([application(1)]));
    mount();
    const list = await rows();
    expect(within(list).getByText(/Backend Engineer ·/)).toBeInTheDocument();
    // On narrow screens each row opens its Application's own page.
    expect(
      within(list).getByRole("link", { name: "Review CV of Candidate 1" }),
    ).toHaveAttribute(
      "href",
      `/recruiter/job-postings/${postingId}/applications/${application(1).id}`,
    );
    expect(applicationsCall("/recruiter/applications")?.[1]).toEqual({
      params: { limit: 20, offset: 0 },
      signal: expect.any(AbortSignal),
    });
  });

  it("sends the URL filters to Go and keeps them in the pagination links", async () => {
    serve(page([application(21)], 41, 20));
    mount(
      {},
      {
        status: "rejected",
        search: "linh",
        page: 2,
        posting: postingId,
      },
    );
    await rows();
    expect(applicationsCall("/recruiter/applications")?.[1]).toEqual({
      params: {
        limit: 20,
        offset: 20,
        status: "rejected",
        q: "linh",
        job_posting_id: postingId,
      },
      signal: expect.any(AbortSignal),
    });
    expect(
      screen.getByRole("combobox", { name: "Job posting" }),
    ).toHaveTextContent("Selected posting");
    expect(screen.getByText("Showing 21–40 of 41")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Previous" })).toHaveAttribute(
      "href",
      `/recruiter/applications?status=rejected&q=linh&posting=${postingId}`,
    );
    expect(screen.getByRole("link", { name: "Next" })).toHaveAttribute(
      "href",
      `/recruiter/applications?status=rejected&q=linh&posting=${postingId}&page=3`,
    );
  });

  it("ignores a posting filter in the URL on a posting's own list", async () => {
    mocks.pathname = `/recruiter/job-postings/${postingId}`;
    serve(page([application(1)]));
    mount(
      { postingId },
      {
        ...defaultApplicationFilters,
        posting: "99999999-9999-4999-8999-999999999999",
      },
    );
    await rows();
    expect(mocks.get).toHaveBeenCalledWith(
      `/job-postings/${postingId}/applications`,
      { params: { limit: 20, offset: 0 }, signal: expect.any(AbortSignal) },
    );
  });

  it("exposes the search form by role and name, with one Search button inside it", async () => {
    serve(page([application(1)]));
    mount();
    await rows();
    // The E2E specs reach the button through this form: the workspace also has a "Search pages"
    // button, so a bare name match is ambiguous there.
    const form = screen.getByRole("search", { name: "Search applications" });
    expect(
      within(form).getByRole("button", { name: "Search" }),
    ).toHaveAttribute("type", "submit");
    expect(
      within(form).getByLabelText("Search by name or email"),
    ).toHaveAttribute("placeholder", "Search name or email");
  });

  it("shows the active status and changes it through the History API", async () => {
    serve(page([application(1)]));
    mount({}, { ...defaultApplicationFilters, status: "approved" });
    await rows();
    const status = screen.getByRole("combobox", { name: "Status" });
    expect(status).toHaveTextContent("Approved");
    const push = vi
      .spyOn(window.history, "pushState")
      .mockImplementation(() => {});
    fireEvent.keyDown(status, { key: "ArrowDown" });
    fireEvent.keyDown(
      await screen.findByRole("option", { name: "Interview eligible" }),
      { key: "Enter" },
    );
    expect(push).toHaveBeenCalledWith(
      null,
      "",
      "/recruiter/applications?status=interview_eligible",
    );
    push.mockRestore();
  });

  it("explains an empty list, and offers a way back when it is filtered", async () => {
    serve(page([]));
    mount();
    expect(await screen.findByText("No applications yet.")).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Show all applications" }),
    ).not.toBeInTheDocument();
  });

  it("lets 'show all applications' drop the one-posting narrowing too", async () => {
    serve(page([]));
    mount(
      {},
      { ...defaultApplicationFilters, search: "zzz", posting: postingId },
    );
    expect(
      await screen.findByText("No applications match “zzz”."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Show all applications" }),
    ).toHaveAttribute("href", "/recruiter/applications");
  });

  it("names the search when nothing matches it", async () => {
    serve(page([]));
    mount({}, { ...defaultApplicationFilters, search: "zzz" });
    expect(
      await screen.findByText("No applications match “zzz”."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Clear search" }),
    ).toBeInTheDocument();
  });

  it("offers the first page when the page is past the end", async () => {
    serve(page([], 5, 40));
    mount({}, { ...defaultApplicationFilters, page: 3 });
    expect(
      await screen.findByText("This page is past the end of the list."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Go to the first page" }),
    ).toHaveAttribute("href", "/recruiter/applications");
  });

  it("shows an error with a retry", async () => {
    serve(failure(500));
    mount();
    expect(
      await screen.findByText("We couldn't load the applications"),
    ).toBeInTheDocument();
    serve(page([application(1)]));
    screen.getByRole("button", { name: "Try again" }).click();
    expect(await rows()).toBeInTheDocument();
  });

  it("treats a missing posting as not found, with no retry", async () => {
    serve(failure(404));
    mount({ postingId });
    expect(
      await screen.findByText("We couldn't find this job posting"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Try again" }),
    ).not.toBeInTheDocument();
  });

  it("re-checks the session when access is denied", async () => {
    serve(failure(403));
    const client = mount();
    expect(
      await screen.findByText("You don't have access to applications"),
    ).toBeInTheDocument();
    expect(client.getQueryState(["auth", "me"])?.isInvalidated).toBe(true);
  });
});
