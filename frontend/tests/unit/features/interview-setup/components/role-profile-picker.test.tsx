import { beforeEach, expect, it, vi } from "vitest";
import { fireEvent, screen, within } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RoleProfilePicker } from "@/features/interview-setup/components/role-profile-picker";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  replace: vi.fn(),
  user: {
    id: "11111111-1111-4111-8111-111111111111",
    full_name: "Cam Candidate",
    email: "cam@example.com",
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
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mocks.replace }),
}));

const frontend = "22222222-2222-4222-8222-000000000001";
const backend = "22222222-2222-4222-8222-000000000002";
const draft = "22222222-2222-4222-8222-000000000003";
const sessionId = "66666666-6666-4666-8666-666666666666";

const summary = (id: string, title: string, status = "confirmed") => ({
  id,
  title,
  seniorityLevel: "mid",
  status,
  createdAt: "2026-10-20T08:00:00Z",
});

function api({
  profiles = [
    summary(frontend, "Frontend Engineer"),
    summary(backend, "Backend Engineer"),
    summary(draft, "Data Engineer", "review"),
  ],
  unfinished = false,
}: { profiles?: ReturnType<typeof summary>[]; unfinished?: boolean } = {}) {
  mocks.get.mockImplementation((url: string) => {
    if (url === "/role-profiles")
      return Promise.resolve({
        data: {
          success: true,
          data: {
            items: profiles,
            pagination: { limit: 20, offset: 0, total: profiles.length },
          },
        },
      });
    if (url === "/interviews")
      return Promise.resolve({
        data: {
          success: true,
          data: {
            items: unfinished
              ? [
                  {
                    id: sessionId,
                    type: "practice",
                    started_at: "2026-10-28T08:00:00Z",
                    completed_at: null,
                    evaluation_status: "pending",
                    status: "in_progress",
                    score: null,
                    role_profile_title: "Backend Engineer",
                    job_posting_title: null,
                    application_id: null,
                  },
                ]
              : [],
            total: unfinished ? 1 : 0,
            page: 1,
            page_size: 100,
          },
        },
      });
    if (url === `/interviews/${sessionId}`)
      return Promise.resolve({
        data: {
          success: true,
          data: {
            id: sessionId,
            type: "practice",
            role_profile_id: backend,
            application_id: null,
            avatar_id: null,
            voice_id: null,
            started_at: "2026-10-28T08:00:00Z",
            completed_at: null,
            evaluation_status: "pending",
            question_count: 8,
            answered_count: 3,
          },
        },
      });
    return Promise.reject(new Error(`unexpected GET ${url}`));
  });
}

function renderPicker(roleProfileId?: string) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return renderWithIntl(
    <QueryClientProvider client={client}>
      <RoleProfilePicker roleProfileId={roleProfileId} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  mocks.get.mockReset();
  mocks.replace.mockReset();
});

it("lists only confirmed Role Profiles and keeps the choice in the URL", async () => {
  api();
  renderPicker();
  const group = await screen.findByRole("radiogroup", {
    name: "Confirmed Role Profiles",
  });
  expect(within(group).getAllByRole("radio")).toHaveLength(2);
  expect(screen.queryByText("Data Engineer")).not.toBeInTheDocument();
  expect(
    screen.getByRole("button", { name: /continue to interviewer/i }),
  ).toBeDisabled();
  fireEvent.click(screen.getByRole("radio", { name: /frontend engineer/i }));
  expect(mocks.replace).toHaveBeenCalledWith(
    `/interviews/new/setup?roleProfileId=${frontend}`,
    { scroll: false },
  );
});

it("continues a selected profile to the interviewer step", async () => {
  api();
  renderPicker(frontend);
  expect(
    await screen.findByRole("radio", { name: /frontend engineer/i }),
  ).toBeChecked();
  expect(
    await screen.findByRole("link", { name: /continue to interviewer/i }),
  ).toHaveAttribute(
    "href",
    `/interviews/new/interviewer?roleProfileId=${frontend}`,
  );
});

it("makes an unfinished interview a distinct, free resume path", async () => {
  api({ unfinished: true });
  renderPicker(backend);
  expect(
    await screen.findByText("You have an unfinished interview for this role"),
  ).toBeInTheDocument();
  expect(screen.getByText("Unfinished interview")).toBeInTheDocument();
  expect(screen.getByText(/3 of 8/)).toBeInTheDocument();
  expect(
    screen.getByText("Resuming is free and keeps its interviewer and voice."),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("link", { name: /resume interview/i }),
  ).toHaveAttribute(
    "href",
    `/interviews/new/preflight?roleProfileId=${backend}`,
  );
  expect(
    screen.queryByRole("link", { name: /continue to interviewer/i }),
  ).not.toBeInTheDocument();
});

it("asks for a confirmed Role Profile when there is none", async () => {
  api({ profiles: [summary(draft, "Data Engineer", "review")] });
  renderPicker();
  expect(
    await screen.findByRole("heading", {
      name: "Choose your Role Profile first.",
    }),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("link", { name: /add a role profile/i }),
  ).toHaveAttribute("href", "/role-profiles/new");
});

it("sends a profile that still needs review back to its page", async () => {
  api();
  renderPicker(draft);
  expect(
    await screen.findByRole("heading", { name: "Confirm this role first." }),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("link", { name: "Return to Role Profile" }),
  ).toHaveAttribute("href", `/role-profiles/${draft}`);
});
