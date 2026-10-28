import { beforeEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { InterviewerPicker } from "@/features/interview-setup/components/interviewer-picker";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  push: vi.fn(),
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
  useRouter: () => ({ push: mocks.push }),
}));

const profileId = "22222222-2222-4222-8222-222222222222";
const mia = "44444444-4444-4444-8444-000000000001";
const duc = "44444444-4444-4444-8444-000000000002";
const ava = "55555555-5555-4555-8555-000000000001";
const retired = "55555555-5555-4555-8555-000000000009";

const avatar = (id: string, name: string) => ({
  id,
  name,
  personality: "Calm and precise; asks for trade-offs.",
  created_at: "2026-10-20T08:00:00Z",
  updated_at: "2026-10-20T08:00:00Z",
});

function api({
  avatars = [avatar(mia, "Mia"), avatar(duc, "Đức")],
}: { avatars?: ReturnType<typeof avatar>[] } = {}) {
  mocks.get.mockImplementation((url: string) => {
    if (url === "/inventory/avatars")
      return Promise.resolve({
        data: {
          success: true,
          data: {
            items: avatars,
            pagination: { limit: 12, offset: 0, total: avatars.length },
          },
        },
      });
    if (url === "/voices")
      return Promise.resolve({
        data: {
          success: true,
          data: {
            items: [{ id: ava, name: "Ava - English" }],
            pagination: { limit: 100, offset: 0, total: 1 },
          },
        },
      });
    if (url === "/interviews")
      return Promise.resolve({
        data: {
          success: true,
          data: { items: [], total: 0, page: 1, page_size: 100 },
        },
      });
    return Promise.reject(new Error(`unexpected GET ${url}`));
  });
}

function renderPicker(props: Parameters<typeof InterviewerPicker>[0]) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <InterviewerPicker {...props} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  mocks.get.mockReset();
  mocks.push.mockReset();
  vi.stubGlobal(
    "IntersectionObserver",
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
      takeRecords() {
        return [];
      }
    },
  );
});

it("defaults to the 2D interviewer and the default voice, and sends neither", async () => {
  api();
  renderPicker({ roleProfileId: profileId });
  expect(
    await screen.findByRole("radio", { name: /2d interviewer/i }),
  ).toBeChecked();
  await screen.findByRole("radio", { name: /mia/i });
  fireEvent.click(
    screen.getByRole("button", { name: /continue to preflight/i }),
  );
  await waitFor(() =>
    expect(mocks.push).toHaveBeenCalledWith(
      `/interviews/new/preflight?roleProfileId=${profileId}`,
    ),
  );
});

it("carries a chosen avatar and voice to the preflight", async () => {
  api();
  renderPicker({ roleProfileId: profileId, voiceId: ava });
  fireEvent.click(await screen.findByRole("radio", { name: /đức/i }));
  expect(screen.getByRole("radio", { name: /đức/i })).toBeChecked();
  expect(
    await screen.findByRole("combobox", { name: "Voice" }),
  ).toHaveTextContent("Ava - English");
  fireEvent.click(
    screen.getByRole("button", { name: /continue to preflight/i }),
  );
  await waitFor(() =>
    expect(mocks.push).toHaveBeenCalledWith(
      `/interviews/new/preflight?roleProfileId=${profileId}&avatarId=${duc}&voiceId=${ava}`,
    ),
  );
});

it("drops an avatar or voice that is gone instead of sending it", async () => {
  api({ avatars: [avatar(mia, "Mia")] });
  renderPicker({ roleProfileId: profileId, avatarId: duc, voiceId: retired });
  expect(
    await screen.findByText(
      /the avatar you chose is no longer in your inventory/i,
    ),
  ).toBeInTheDocument();
  expect(
    await screen.findByText(/the voice you chose is no longer available/i),
  ).toBeInTheDocument();
  expect(screen.getByRole("radio", { name: /2d interviewer/i })).toBeChecked();
  fireEvent.click(
    screen.getByRole("button", { name: /continue to preflight/i }),
  );
  await waitFor(() =>
    expect(mocks.push).toHaveBeenCalledWith(
      `/interviews/new/preflight?roleProfileId=${profileId}`,
    ),
  );
});

it("still offers the 2D interviewer with an empty inventory", async () => {
  api({ avatars: [] });
  renderPicker({ roleProfileId: profileId });
  expect(
    await screen.findByText(/you don't have an avatar yet/i),
  ).toBeInTheDocument();
  expect(screen.getByRole("radio", { name: /2d interviewer/i })).toBeChecked();
  expect(
    screen.getByRole("link", { name: "Create one in Avatar Studio" }),
  ).toHaveAttribute("href", "/avatar-studio/new");
});

it("asks for a Role Profile first when none is chosen", () => {
  api();
  renderPicker({});
  expect(
    screen.getByRole("heading", { name: "Choose your Role Profile first." }),
  ).toBeInTheDocument();
});
