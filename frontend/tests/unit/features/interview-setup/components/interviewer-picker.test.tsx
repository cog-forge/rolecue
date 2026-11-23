import { beforeEach, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  avaVoiceId as ava,
  ducId as duc,
  envelope,
  miaId as mia,
  practiceView,
  profileId,
} from "../saved-setup-fixtures";
import { InterviewerPicker } from "@/features/interview-setup/components/interviewer-picker";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  put: vi.fn(),
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
vi.mock("@/lib/api/client", () => ({
  apiClient: { get: mocks.get, put: mocks.put },
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mocks.push }),
}));

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
  saved = practiceView(),
}: {
  avatars?: ReturnType<typeof avatar>[];
  saved?: ReturnType<typeof practiceView>;
} = {}) {
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
    if (url === `/interview-setups/practice/${profileId}`)
      return Promise.resolve(envelope(saved));
    return Promise.reject(new Error(`unexpected GET ${url}`));
  });
  // The API answers a save with the revalidated view of what it stored.
  mocks.put.mockImplementation((_url: string, body: Record<string, string>) =>
    Promise.resolve(
      envelope(
        practiceView({
          saved: true,
          avatar: body.avatar_id ? { id: body.avatar_id, name: "Mia" } : null,
          voice: body.voice_id
            ? { id: body.voice_id, name: "Ava - English" }
            : null,
          environment: body.environment,
        }),
      ),
    ),
  );
}

function renderPicker(props: Parameters<typeof InterviewerPicker>[0]) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return renderWithIntl(
    <QueryClientProvider client={client}>
      <InterviewerPicker {...props} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  mocks.get.mockReset();
  mocks.put.mockReset();
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

const putPath = `/interview-setups/practice/${profileId}`;

it("pre-fills avatar, voice and room from the saved setup", async () => {
  api({
    saved: practiceView({
      saved: true,
      avatar: { id: duc, name: "Đức" },
      voice: { id: ava, name: "Ava - English" },
      environment: "meeting-3d",
    }),
  });
  renderPicker({ roleProfileId: profileId });
  expect(await screen.findByRole("radio", { name: /đức/i })).toBeChecked();
  expect(screen.getByRole("combobox", { name: "Voice" })).toHaveTextContent(
    "Ava - English",
  );
  expect(
    screen.getByRole("radio", { name: "Meeting room (3D room)" }),
  ).toBeChecked();
});

it("lets the saved setup win over the URL once it exists", async () => {
  api({
    saved: practiceView({
      saved: true,
      avatar: { id: mia, name: "Mia" },
      voice: null,
      environment: "office",
    }),
  });
  renderPicker({ roleProfileId: profileId, avatarId: duc, voiceId: ava });
  expect(await screen.findByRole("radio", { name: /mia/i })).toBeChecked();
  expect(screen.getByRole("radio", { name: /đức/i })).not.toBeChecked();
  expect(screen.getByRole("combobox", { name: "Voice" })).toHaveTextContent(
    "Choose a voice",
  );
});

it("seeds a never-saved setup from the URL", async () => {
  api();
  renderPicker({ roleProfileId: profileId, avatarId: duc, voiceId: ava });
  expect(await screen.findByRole("radio", { name: /đức/i })).toBeChecked();
  expect(screen.getByRole("combobox", { name: "Voice" })).toHaveTextContent(
    "Ava - English",
  );
});

it("saves the full replace body on every choice", async () => {
  api();
  renderPicker({ roleProfileId: profileId });
  fireEvent.click(await screen.findByRole("radio", { name: /mia/i }));
  await waitFor(() =>
    expect(mocks.put).toHaveBeenLastCalledWith(putPath, {
      avatar_id: mia,
      voice_id: null,
      environment: null,
    }),
  );
  fireEvent.click(screen.getByRole("radio", { name: "Balcony" }));
  await waitFor(() =>
    expect(mocks.put).toHaveBeenLastCalledWith(putPath, {
      avatar_id: mia,
      voice_id: null,
      environment: "balcony",
    }),
  );
  expect(await screen.findByText("Saved")).toBeInTheDocument();
});

it("saves, then carries the choices to the preflight", async () => {
  api();
  renderPicker({ roleProfileId: profileId });
  fireEvent.click(await screen.findByRole("radio", { name: /đức/i }));
  fireEvent.click(screen.getByRole("radio", { name: "Balcony" }));
  fireEvent.click(
    screen.getByRole("button", { name: /continue to preflight/i }),
  );
  await waitFor(() =>
    expect(mocks.push).toHaveBeenCalledWith(
      `/interviews/new/preflight?roleProfileId=${profileId}&avatarId=${duc}`,
    ),
  );
  expect(mocks.put).toHaveBeenLastCalledWith(putPath, {
    avatar_id: duc,
    voice_id: null,
    environment: "balcony",
  });
});

it("shows a failed save with a retry and stays on the step", async () => {
  api();
  renderPicker({ roleProfileId: profileId });
  mocks.put.mockRejectedValueOnce(new Error("offline"));
  fireEvent.click(await screen.findByRole("radio", { name: /mia/i }));
  expect(
    await screen.findByText("We couldn't save your choices."),
  ).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Try again" }));
  expect(await screen.findByText("Saved")).toBeInTheDocument();
  expect(mocks.put).toHaveBeenCalledTimes(2);
});

it("does not continue while the save fails", async () => {
  api();
  renderPicker({ roleProfileId: profileId });
  await screen.findByRole("radio", { name: /mia/i });
  mocks.put.mockRejectedValue(new Error("offline"));
  fireEvent.click(
    screen.getByRole("button", { name: /continue to preflight/i }),
  );
  expect(
    await screen.findByText("We couldn't save your choices."),
  ).toBeInTheDocument();
  expect(mocks.push).not.toHaveBeenCalled();
});

it("drops an avatar or voice that is gone instead of saving it", async () => {
  api({
    avatars: [avatar(mia, "Mia")],
    saved: practiceView({
      saved: true,
      avatar: { id: duc, name: "Đức" },
      voice: { id: retired, name: "Old" },
      environment: "cafe",
    }),
  });
  renderPicker({ roleProfileId: profileId });
  expect(
    await screen.findByText(
      /the avatar you chose is no longer in your inventory/i,
    ),
  ).toBeInTheDocument();
  expect(
    await screen.findByText(/the voice you chose is no longer available/i),
  ).toBeInTheDocument();
  fireEvent.click(
    screen.getByRole("button", { name: /continue to preflight/i }),
  );
  await waitFor(() =>
    expect(mocks.put).toHaveBeenCalledWith(putPath, {
      avatar_id: null,
      voice_id: null,
      environment: "cafe",
    }),
  );
});

it("invites an empty inventory to the Studio", async () => {
  api({ avatars: [] });
  renderPicker({ roleProfileId: profileId });
  expect(
    await screen.findByText(/you don't have an avatar yet/i),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("link", { name: "Create one in Avatar Studio" }),
  ).toHaveAttribute("href", "/avatar-studio/new");
});

it("offers a retry when the saved setup cannot be loaded", async () => {
  api();
  mocks.get.mockImplementation((url: string) =>
    url === putPath
      ? Promise.reject(new Error("down"))
      : Promise.resolve({
          data: {
            success: true,
            data: {
              items: [],
              total: 0,
              page: 1,
              page_size: 100,
              pagination: { limit: 12, offset: 0, total: 0 },
            },
          },
        }),
  );
  renderPicker({ roleProfileId: profileId });
  expect(
    await screen.findByText("We couldn't load your saved setup."),
  ).toBeInTheDocument();
});

it("asks for a Role Profile first when none is chosen", () => {
  api();
  renderPicker({});
  expect(
    screen.getByRole("heading", { name: "Choose your Role Profile first." }),
  ).toBeInTheDocument();
});
