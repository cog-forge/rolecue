import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AdminAccessAudit } from "@/features/admin/access-audit/components/admin-access-audit";

const mocks = vi.hoisted(() => ({
  list: vi.fn(),
  user: { id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", role: "admin" },
}));

vi.mock("@/lib/api/client", () => ({
  apiClient: {
    get: async (_url: string, options: { params?: unknown }) => ({
      data: { success: true, data: await mocks.list(options.params) },
    }),
  },
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));

const recruiterId = "11111111-1111-4111-8111-111111111111";
const applicationId = "22222222-2222-4222-8222-222222222222";
const interviewId = "33333333-3333-4333-8333-333333333333";

const entry = (id: string, action: string) => ({
  id,
  recruiter_id: recruiterId,
  application_id: applicationId,
  interview_id: interviewId,
  action,
  created_at: "2026-11-05T08:00:00Z",
});

function renderScreen(locale: "en" | "vi" = "en") {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return renderWithIntl(
    <QueryClientProvider client={client}>
      <AdminAccessAudit />
    </QueryClientProvider>,
    { locale },
  );
}

describe("AdminAccessAudit", () => {
  beforeEach(() => {
    mocks.list.mockReset();
    mocks.list.mockResolvedValue({
      items: [
        entry("44444444-4444-4444-8444-444444444444", "transcript_view"),
        entry("55555555-5555-4555-8555-555555555555", "recording_view"),
      ],
      total: 2,
      page: 1,
      page_size: 20,
    });
  });

  it("lists who read what, with IDs and times only", async () => {
    renderScreen();
    const table = await screen.findByRole("table", {
      name: "Recruiter access audit",
    });
    expect(within(table).getByText("Read a transcript")).toBeVisible();
    expect(within(table).getByText("Opened a recording")).toBeVisible();
    expect(within(table).getAllByText(recruiterId)).toHaveLength(2);
    expect(screen.getByText("2 entries")).toBeVisible();
    expect(mocks.list).toHaveBeenCalledWith({ page: 1, page_size: 20 });
  });

  it("filters by recruiter and application and refuses a malformed ID", async () => {
    renderScreen();
    await screen.findByRole("table");
    fireEvent.change(screen.getByLabelText("Recruiter ID"), {
      target: { value: "not-an-id" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Apply filters" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("full ID");
    expect(mocks.list).toHaveBeenCalledTimes(1);

    fireEvent.change(screen.getByLabelText("Recruiter ID"), {
      target: { value: recruiterId },
    });
    fireEvent.change(screen.getByLabelText("Application ID"), {
      target: { value: applicationId },
    });
    fireEvent.click(screen.getByRole("button", { name: "Apply filters" }));
    await waitFor(() =>
      expect(mocks.list).toHaveBeenLastCalledWith({
        page: 1,
        page_size: 20,
        recruiter_id: recruiterId,
        application_id: applicationId,
      }),
    );
  });

  it("shows an empty state", async () => {
    mocks.list.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    });
    renderScreen();
    expect(await screen.findByText("No access recorded yet")).toBeVisible();
  });

  it("hides the audit when the API refuses the caller", async () => {
    mocks.list.mockRejectedValue(
      Object.assign(new Error("forbidden"), {
        isAxiosError: true,
        response: { status: 403, data: { success: false } },
      }),
    );
    renderScreen();
    expect(
      await screen.findByText(/couldn't confirm your administrator access/),
    ).toBeVisible();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("speaks Vietnamese", async () => {
    renderScreen("vi");
    expect(
      await screen.findByRole("heading", { name: "Nhật ký truy cập" }),
    ).toBeVisible();
    expect(await screen.findByText("Đã đọc bản ghi lời thoại")).toBeVisible();
  });
});
