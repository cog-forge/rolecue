import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { AvatarInventory } from "@/features/avatar/components/avatar-inventory";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  delete: vi.fn(),
  push: vi.fn(),
  params: new URLSearchParams(),
  toast: vi.fn(),
  portrait: vi.fn(),
  webgl: true,
  user: { id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", role: "candidate" },
}));
vi.mock("@/lib/api/client", () => ({
  apiClient: { get: mocks.get, post: mocks.post, delete: mocks.delete },
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mocks.push }),
  usePathname: () => "/avatar-studio",
  useSearchParams: () => mocks.params,
}));
vi.mock("goey-toast", () => ({ gooeyToast: { success: mocks.toast } }));
vi.mock("@/components/avatar-viewer/avatar-viewer", () => ({
  AvatarViewer: ({ label }: { label: string }) => <div>{label}</div>,
}));
// Portraits need WebGL; the stand-ins let each test decide what the renderer returns.
vi.mock("@/components/avatar-viewer/webgl", () => ({
  isWebGLAvailable: () => mocks.webgl,
}));
vi.mock("@/components/avatar-viewer/render-portrait", () => ({
  renderAvatarPortrait: mocks.portrait,
}));

const avatar = (n: number) => ({
  id: `1111111${n}-1111-4111-8111-111111111111`,
  name: `Avatar ${n}`,
  personality: `Personality ${n}`,
  created_at: "2026-10-27T10:00:00Z",
  updated_at: "2026-10-27T10:00:00Z",
});
let summary = { capacity: 3, used: 2, available: 1, is_full: false };
let items = [avatar(1), avatar(2)];
let balance = 5;

function apiError(status: number, code: string) {
  return new AxiosError(
    "Request failed",
    "ERR_BAD_RESPONSE",
    undefined,
    undefined,
    {
      status,
      statusText: "",
      data: { success: false, error: { code, message: code } },
      headers: {},
      config: { headers: new AxiosHeaders() },
    },
  );
}

function renderInventory() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <AvatarInventory />
    </QueryClientProvider>,
  );
}

const slots = () =>
  within(screen.getByRole("list", { name: /^Avatar slots/ })).getAllByRole(
    "listitem",
  );

beforeEach(() => {
  vi.clearAllMocks();
  mocks.params = new URLSearchParams();
  mocks.webgl = true;
  mocks.user = {
    id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
    role: "candidate",
  };
  summary = { capacity: 3, used: 2, available: 1, is_full: false };
  items = [avatar(1), avatar(2)];
  balance = 5;
  mocks.portrait.mockImplementation(
    async (access: { url: string }) => `blob:${access.url}`,
  );
  mocks.get.mockImplementation(async (url: string) => {
    if (url === "/inventory") return { data: { success: true, data: summary } };
    if (url === "/inventory/avatars")
      return {
        data: {
          success: true,
          data: {
            items,
            pagination: { limit: 12, offset: 0, total: items.length },
          },
        },
      };
    const detail = url.match(/^\/inventory\/avatars\/(.+)$/);
    if (detail)
      return {
        data: {
          success: true,
          data: {
            ...items.find((item) => item.id === detail[1]),
            model_access: {
              url: `https://s3.test/${detail[1]}.vrm`,
              method: "GET",
              headers: {},
              expires_at: "2026-10-27T10:05:00Z",
            },
          },
        },
      };
    if (url === "/wallet")
      return {
        data: {
          success: true,
          data: { id: "w", user_id: mocks.user.id, balance },
        },
      };
    if (url === "/wallet/packages")
      return { data: { success: true, data: [] } };
    throw new Error(`unexpected GET ${url}`);
  });
});

describe("avatar inventory", () => {
  it("shows one card per slot the API reports: avatars first, then empty slots", async () => {
    summary = { capacity: 5, used: 2, available: 3, is_full: false };
    renderInventory();
    expect(
      await screen.findByRole("list", {
        name: "Avatar slots, 2 of 5 used",
      }),
    ).toBeInTheDocument();
    expect(slots()).toHaveLength(5);
    expect(
      screen.getByText("3 free slots for new avatars."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", {
        name: "Create an avatar in empty slot 3 of 5",
      }),
    ).toHaveAttribute("href", "/avatar-studio/new");
    expect(screen.getByRole("link", { name: "Create avatar" })).toHaveAttribute(
      "href",
      "/avatar-studio/new",
    );
  });

  it("renders each avatar's 3D portrait into its card from a fresh grant", async () => {
    const { container } = renderInventory();
    await waitFor(() => expect(mocks.portrait).toHaveBeenCalledTimes(2));
    expect(mocks.portrait).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `https://s3.test/${avatar(1).id}.vrm`,
        method: "GET",
      }),
      expect.any(AbortSignal),
    );
    await waitFor(() =>
      expect(container.querySelectorAll('img[src^="blob:"]')).toHaveLength(2),
    );
  });

  it("falls back to a 2D card when WebGL is unavailable, without loading models", async () => {
    mocks.webgl = false;
    renderInventory();
    expect(
      await screen.findAllByText("3D isn't available on this device"),
    ).toHaveLength(2);
    expect(mocks.portrait).not.toHaveBeenCalled();
  });

  it("invites the first creation when the inventory is empty", async () => {
    items = [];
    summary = { capacity: 3, used: 0, available: 3, is_full: false };
    renderInventory();
    expect(
      await screen.findByRole("link", {
        name: "Create an avatar in empty slot 1 of 3",
      }),
    ).toBeInTheDocument();
    expect(slots()).toHaveLength(3);
  });

  it("when full, offers delete and buy separately, with no generation fee", async () => {
    summary = { capacity: 2, used: 2, available: 0, is_full: true };
    renderInventory();
    expect(await screen.findByText("Full")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Delete an avatar you no longer need, or buy more slots to create another.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Create avatar" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Buy slots" })).toBeEnabled();
    expect(screen.getByText("1 coin per slot")).toBeInTheDocument();
    expect(slots()).toHaveLength(3); // two avatars and the "Add more slots" card
    expect(screen.queryByText(/generation/i)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Add more slots/ }));
    expect(
      await screen.findByRole("dialog", { name: "Buy avatar slots" }),
    ).toBeInTheDocument();
  });

  it("opens the 3D view through the URL", async () => {
    renderInventory();
    fireEvent.click(
      await screen.findByRole("button", { name: "Open Avatar 1 in 3D" }),
    );
    expect(mocks.push).toHaveBeenCalledWith(
      `/avatar-studio?avatar=${avatar(1).id}`,
      { scroll: false },
    );
  });

  it("explains an avatar that is in use and keeps it", async () => {
    mocks.delete.mockRejectedValue(apiError(409, "AVATAR_IN_USE"));
    renderInventory();
    fireEvent.click(
      await screen.findByRole("button", { name: "Delete Avatar 1" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Delete avatar" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "used by a job posting or an interview",
    );
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("deletes an unused avatar and refreshes the inventory", async () => {
    mocks.delete.mockResolvedValue({ status: 204, data: "" });
    renderInventory();
    fireEvent.click(
      await screen.findByRole("button", { name: "Delete Avatar 2" }),
    );
    items = [avatar(1)];
    summary = { capacity: 3, used: 1, available: 2, is_full: false };
    fireEvent.click(screen.getByRole("button", { name: "Delete avatar" }));
    expect(
      await screen.findByRole("list", { name: "Avatar slots, 1 of 3 used" }),
    ).toBeInTheDocument();
    expect(mocks.toast).toHaveBeenCalledWith(
      "Avatar deleted",
      expect.anything(),
    );
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("refreshes the list when a deletion can't be confirmed", async () => {
    mocks.delete.mockRejectedValue(
      new AxiosError("Network Error", "ERR_NETWORK"),
    );
    renderInventory();
    fireEvent.click(
      await screen.findByRole("button", { name: "Delete Avatar 1" }),
    );
    const before = mocks.get.mock.calls.length;
    fireEvent.click(screen.getByRole("button", { name: "Delete avatar" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "couldn't confirm the deletion",
    );
    await waitFor(() =>
      expect(mocks.get.mock.calls.length).toBeGreaterThan(before),
    );
  });
});

describe("buy slots", () => {
  async function openDialog() {
    renderInventory();
    await screen.findByRole("list", { name: /^Avatar slots/ });
    fireEvent.click(screen.getByRole("button", { name: "Buy slots" }));
    return screen.findByRole("dialog", { name: "Buy avatar slots" });
  }

  it("prices slots at 1 coin and applies the summary from a 200", async () => {
    mocks.post.mockResolvedValue({
      status: 200,
      data: {
        success: true,
        data: {
          capacity: 5,
          used: 2,
          available: 3,
          is_full: false,
          balance: 3,
        },
      },
    });
    const dialog = await openDialog();
    expect(await within(dialog).findByText("5 coins")).toBeInTheDocument();
    fireEvent.click(
      within(dialog).getByRole("button", { name: "One slot more" }),
    );
    expect(within(dialog).getByText("2 coins")).toBeInTheDocument();
    expect(within(dialog).getByText("5 slots")).toBeInTheDocument();
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Buy for 2 coins" }),
    );
    expect(
      await screen.findByRole("list", { name: "Avatar slots, 2 of 5 used" }),
    ).toBeInTheDocument();
    expect(mocks.post).toHaveBeenCalledTimes(1);
    expect(mocks.post).toHaveBeenCalledWith("/inventory/purchase", {
      amount: 2,
    });
  });

  it("blocks a purchase the balance can't cover and opens the coin packages", async () => {
    balance = 0;
    mocks.user = {
      id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
      role: "recruiter",
    };
    const dialog = await openDialog();
    expect(
      await within(dialog).findByText(/You need 1 coin more/),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByRole("button", { name: "Buy for 1 coin" }),
    ).toBeDisabled();
    fireEvent.click(within(dialog).getByRole("button", { name: "Add coins" }));
    expect(
      await screen.findByRole("dialog", { name: "Add coins" }),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(mocks.get).toHaveBeenCalledWith("/wallet/packages", {
        signal: expect.any(AbortSignal),
      }),
    );
  });

  it("shows an insufficient balance rejection", async () => {
    mocks.post.mockRejectedValue(apiError(422, "INSUFFICIENT_BALANCE"));
    const dialog = await openDialog();
    await within(dialog).findByText("5 coins");
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Buy for 1 coin" }),
    );
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "don't have enough coins",
    );
    expect(
      within(dialog).getByRole("button", { name: "Add coins" }),
    ).toBeInTheDocument();
  });

  it("refreshes and never resends when the purchase outcome is unknown", async () => {
    mocks.post.mockRejectedValue(new AxiosError("timeout", "ECONNABORTED"));
    const dialog = await openDialog();
    await within(dialog).findByText("5 coins");
    const before = mocks.get.mock.calls.filter(
      ([url]) => url === "/inventory",
    ).length;
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Buy for 1 coin" }),
    );
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "couldn't confirm the purchase",
    );
    await waitFor(() =>
      expect(
        mocks.get.mock.calls.filter(([url]) => url === "/inventory").length,
      ).toBeGreaterThan(before),
    );
    expect(mocks.post).toHaveBeenCalledTimes(1);
  });
});
