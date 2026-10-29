import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { jobBoardKeys } from "@/features/job-board/api/job-board";
import { JobPostingDetail } from "@/features/job-board/components/job-posting-detail";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
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
vi.mock("@/lib/api/client", () => ({ apiClient: { get: mocks.get } }));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));

const id = "33333333-3333-4333-8333-333333333333";
const detail = (extra: Record<string, unknown> = {}) => ({
  data: {
    success: true,
    data: {
      id,
      title: "Go Backend Engineer",
      company_name: "Acme",
      company_website: "https://acme.example.com",
      location: "Hanoi",
      employment_type: "Full-time",
      salary_min: 10_000_000,
      salary_max: 20_000_000,
      seniority_level: "mid",
      interview_duration: 30,
      created_at: "2026-10-12T08:00:00Z",
      raw_text: "We build APIs.\n\n<b>Bold?</b> <img src=x onerror=alert(1)>",
      requirements: [
        { name: "Go", target: "programming_language", type: "required" },
        { name: "PostgreSQL", target: "database", type: "required" },
        { name: "Kubernetes", target: "cloud_platform", type: "preferred" },
      ],
      already_applied: false,
      ...extra,
    },
  },
});
function failure(status: number) {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: { success: false, error: { code: "X", message: "server says" } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}
function mount(
  backQuery = "",
  postingId = id,
  seed?: unknown,
  variant?: "page" | "panel",
) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  client.setQueryData(["auth", "me"], mocks.user);
  if (seed) client.setQueryData(jobBoardKeys.detail(mocks.user.id, id), seed);
  const view = renderWithIntl(
    <QueryClientProvider client={client}>
      <JobPostingDetail
        id={postingId}
        backQuery={backQuery}
        variant={variant}
      />
    </QueryClientProvider>,
  );
  return { client, ...view };
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
  Object.assign(mocks.user, { role: "candidate" });
});

describe("JobPostingDetail", () => {
  it("loads, then shows the public posting", async () => {
    mocks.get.mockResolvedValue(detail());
    mount();
    expect(screen.getByRole("status")).toHaveTextContent(
      /loading the job posting/i,
    );
    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Go Backend Engineer",
      }),
    ).toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledWith(`/job-board/${id}`, {
      signal: expect.any(AbortSignal),
    });
    const company = screen.getByRole("link", { name: /Acme/ });
    expect(company).toHaveAttribute("href", "https://acme.example.com/");
    expect(company).toHaveAttribute("target", "_blank");
    expect(company).toHaveAttribute("rel", "noopener noreferrer");
    expect(screen.getByText("Hanoi")).toBeInTheDocument();
    expect(screen.getByText("Mid-level")).toBeInTheDocument();
    expect(screen.getByText("10,000,000 – 20,000,000")).toBeInTheDocument();
    expect(screen.getByText("30 minutes")).toBeInTheDocument();
  });

  it("groups skills into required and nice to have", async () => {
    mocks.get.mockResolvedValue(detail());
    mount();
    const required = await screen.findByRole("list", { name: "Required" });
    const preferred = screen.getByRole("list", { name: "Nice to have" });
    const items = within(required).getAllByRole("listitem");
    expect(items).toHaveLength(2);
    expect(items[0]).toHaveTextContent("Go");
    expect(items[0]).toHaveAttribute("title", "Programming language");
    expect(required).toHaveTextContent("PostgreSQL");
    expect(preferred).toHaveTextContent("Kubernetes");
    expect(
      screen.queryByRole("button", { name: /show all/i }),
    ).not.toBeInTheDocument();
  });

  it("shows ten skills first and the rest on request", async () => {
    const requirements = Array.from({ length: 13 }, (_, i) => ({
      name: `Skill ${i + 1}`,
      target: "tool",
      type: i < 11 ? "required" : "preferred",
    }));
    mocks.get.mockResolvedValue(detail({ requirements }));
    mount();
    const required = await screen.findByRole("list", { name: "Required" });
    expect(within(required).getAllByRole("listitem")).toHaveLength(10);
    expect(
      screen.queryByRole("list", { name: "Nice to have" }),
    ).not.toBeInTheDocument();
    const more = screen.getByRole("button", { name: "Show all 13 skills" });
    expect(more).toHaveAttribute("aria-expanded", "false");
    fireEvent.click(more);
    expect(within(required).getAllByRole("listitem")).toHaveLength(11);
    expect(
      within(screen.getByRole("list", { name: "Nice to have" })).getAllByRole(
        "listitem",
      ),
    ).toHaveLength(2);
  });

  it("clamps a long job description until it is expanded", async () => {
    mocks.get.mockResolvedValue(detail({ raw_text: "Line\n".repeat(30) }));
    mount();
    const toggle = await screen.findByRole("button", {
      name: "Show the full description",
    });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    fireEvent.click(toggle);
    expect(screen.getByRole("button", { name: "Show less" })).toHaveAttribute(
      "aria-expanded",
      "true",
    );
  });

  it("explains how hiring works", async () => {
    mocks.get.mockResolvedValue(detail());
    mount();
    expect(
      await screen.findByRole("heading", { name: "How hiring works" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Complete the AI interview within 24 hours"),
    ).toBeInTheDocument();
  });

  it("renders as a side panel with an h2 title and a link to the posting page", async () => {
    mocks.get.mockResolvedValue(detail());
    mount("", id, undefined, "panel");
    expect(
      await screen.findByRole("heading", {
        level: 2,
        name: "Go Backend Engineer",
      }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Back to Job Board" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Open posting page" }),
    ).toHaveAttribute("href", `/jobs/${id}`);
  });

  it("renders the job description as plain text, never as HTML", async () => {
    mocks.get.mockResolvedValue(detail());
    const { container } = mount();
    expect(
      await screen.findByText(/<b>Bold\?<\/b> <img src=x onerror=alert\(1\)>/),
    ).toBeInTheDocument();
    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("b")).toBeNull();
  });

  it("opens the apply dialog for a Candidate who has not applied", async () => {
    mocks.get.mockResolvedValue(detail());
    mount();
    fireEvent.click(
      await screen.findByRole("button", { name: "Apply with your CV" }),
    );
    expect(
      screen.getByRole("dialog", { name: "Apply for Go Backend Engineer" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Choose your CV (PDF)" }),
    ).toBeEnabled();
    expect(screen.queryByText(/already applied/i)).not.toBeInTheDocument();
  });

  it("does not offer the CV upload to a Recruiter", async () => {
    Object.assign(mocks.user, { role: "recruiter" });
    mocks.get.mockResolvedValue(detail());
    mount();
    await screen.findByRole("heading", { level: 1 });
    expect(
      screen.queryByRole("button", { name: "Apply with your CV" }),
    ).not.toBeInTheDocument();
  });

  it("links an applied posting to the Candidate's own application", async () => {
    const applicationId = "44444444-4444-4444-8444-444444444444";
    mocks.get.mockResolvedValue(
      detail({ already_applied: true, application_id: applicationId }),
    );
    mount();
    expect(
      await screen.findByRole("link", { name: "Applied · View application" }),
    ).toHaveAttribute("href", `/applications/${applicationId}`);
    expect(
      screen.queryByRole("button", { name: "Apply with your CV" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent(
      "You already applied to this posting.",
    );
  });

  it("falls back to My Applications when the application id is missing", async () => {
    mocks.get.mockResolvedValue(detail({ already_applied: true }));
    mount();
    expect(
      await screen.findByRole("link", { name: "Applied · View application" }),
    ).toHaveAttribute("href", "/applications");
  });

  it("re-checks Applied when an earlier visit is still cached", async () => {
    mocks.get.mockResolvedValue(detail({ already_applied: true }));
    mount("", id, detail().data.data);
    // The cached copy shows first, then the fresh answer replaces it.
    expect(
      screen.getByRole("button", { name: "Apply with your CV" }),
    ).toBeInTheDocument();
    expect(
      await screen.findByRole("link", { name: "Applied · View application" }),
    ).toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledTimes(1);
  });

  it("never shows recruiter-only data even if the API sent it", async () => {
    mocks.get.mockResolvedValue(
      detail({
        interview_slot: 7319,
        remaining_slots: 4421,
        core_question_count: 5813,
        questions: [{ id, content: "Hidden core question about channels" }],
        evaluation_weights: [{ metric: "Hidden metric", weight: 6217 }],
        reject_reason: "Hidden reject reason",
        required_avatar: { id, name: "Hidden Avatar" },
        required_voice: { id, name: "Hidden Voice" },
      }),
    );
    const { container } = mount();
    await screen.findByRole("heading", { level: 1 });
    for (const value of [
      "7319",
      "4421",
      "5813",
      "6217",
      "Hidden core question",
      "Hidden metric",
      "Hidden reject reason",
      "Hidden Avatar",
      "Hidden Voice",
    ])
      expect(container).not.toHaveTextContent(value);
    expect(
      screen.queryByText(/question bank|capacity|slot|weight/i),
    ).toBeNull();
  });

  it("handles optional fields that are missing", async () => {
    mocks.get.mockResolvedValue(
      detail({
        company_name: null,
        company_website: null,
        location: null,
        employment_type: null,
        salary_min: null,
        salary_max: null,
        interview_duration: null,
        raw_text: "  ",
        requirements: [],
      }),
    );
    mount();
    await screen.findByRole("heading", { level: 1 });
    expect(
      screen.queryByRole("link", { name: /Acme/ }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Salary")).not.toBeInTheDocument();
    expect(
      screen.getByText(/doesn't list specific skills/),
    ).toBeInTheDocument();
    expect(
      screen.getByText("The recruiter hasn't added a job description."),
    ).toBeInTheDocument();
  });

  it("links the website by its host when the company name is missing", async () => {
    mocks.get.mockResolvedValue(detail({ company_name: null }));
    mount();
    expect(
      await screen.findByRole("link", { name: /acme\.example\.com/ }),
    ).toHaveAttribute("href", "https://acme.example.com/");
  });

  it.each([
    "javascript:alert(1)",
    "http://acme.example.com",
    "https://user:pass@acme.example.com",
    `https://acme.example.com/${"a".repeat(2048)}`,
    "not a url",
  ])("does not link an unsafe company website (%s)", async (website) => {
    mocks.get.mockResolvedValue(detail({ company_website: website }));
    mount();
    await screen.findByRole("heading", { level: 1 });
    expect(screen.getByText("Acme")).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: /Acme/ }),
    ).not.toBeInTheDocument();
  });

  it("keeps the board filters in the back link", async () => {
    mocks.get.mockResolvedValue(detail());
    mount("q=Go&page=2");
    expect(
      screen.getByRole("link", { name: "Back to Job Board" }),
    ).toHaveAttribute("href", "/jobs?q=Go&page=2");
  });

  it("explains a mistyped link without asking the API", async () => {
    mount("q=Go", "not-a-uuid");
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("This link isn't valid");
    expect(
      within(alert).getByRole("link", { name: "Browse open postings" }),
    ).toHaveAttribute("href", "/jobs?q=Go");
    expect(mocks.get).not.toHaveBeenCalled();
  });

  it("explains a closed or removed posting (404) without the server message", async () => {
    mocks.get.mockRejectedValue(failure(404));
    mount("q=Go");
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("This posting is no longer available");
    expect(alert).toHaveTextContent(/stopped accepting applications/);
    expect(alert).not.toHaveTextContent("server says");
    expect(
      within(alert).getByRole("link", { name: "Browse open postings" }),
    ).toHaveAttribute("href", "/jobs?q=Go");
    expect(
      screen.queryByRole("button", { name: /apply/i }),
    ).not.toBeInTheDocument();
  });

  it("shows an error with a retry", async () => {
    mocks.get
      .mockRejectedValueOnce(failure(500))
      .mockResolvedValueOnce(detail());
    mount();
    expect(await screen.findByRole("alert")).toHaveTextContent(
      /couldn't load this job posting/i,
    );
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Go Backend Engineer",
      }),
    ).toBeInTheDocument();
  });

  it("treats 403 as an access problem and re-checks the session", async () => {
    mocks.get.mockRejectedValue(failure(403));
    const { client } = mount();
    const invalidate = vi.spyOn(client, "invalidateQueries");
    expect(await screen.findByRole("alert")).toHaveTextContent(
      /don't have access to this posting/i,
    );
    await waitFor(() =>
      expect(invalidate).toHaveBeenCalledWith({ queryKey: ["auth", "me"] }),
    );
    expect(
      screen.queryByRole("button", { name: /apply/i }),
    ).not.toBeInTheDocument();
  });
});
