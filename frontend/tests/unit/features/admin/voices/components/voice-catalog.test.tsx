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
import { VoiceCatalog } from "@/features/admin/voices/components/voice-catalog";
import {
  defaultVoiceFilters,
  type VoiceFilters,
} from "@/features/admin/voices/utils/voice-filters";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  delete: vi.fn(),
  patch: vi.fn(),
  push: vi.fn(),
  toast: vi.fn(),
  user: {
    id: "11111111-1111-4111-8111-111111111111",
    full_name: "Admin",
    email: "admin@example.com",
    role: "admin" as const,
    email_verified: true,
    is_locked: false,
    image: null,
  },
}));
vi.mock("@/lib/api/client", () => ({
  apiClient: { get: mocks.get, delete: mocks.delete, patch: mocks.patch },
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: mocks.push }) }));
vi.mock("goey-toast", () => ({ gooeyToast: { success: mocks.toast } }));

const used = {
  id: "33333333-3333-4333-8333-333333333331",
  name: "Ava",
  is_active: true,
  interview_count: 2,
  job_posting_count: 1,
  in_use: true,
};
const unused = {
  ...used,
  id: "33333333-3333-4333-8333-333333333332",
  name: "Ben",
  interview_count: 0,
  job_posting_count: 0,
  in_use: false,
};
const retired = {
  ...used,
  id: "33333333-3333-4333-8333-333333333333",
  name: "Cara",
  is_active: false,
};
const page = (items: unknown[], total = items.length) => ({
  data: {
    success: true,
    data: { items, pagination: { limit: 20, offset: 0, total } },
  },
});
function failure(status: number, code: string, message: string) {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: { success: false, error: { code, message } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}
function mount(filters: VoiceFilters = defaultVoiceFilters) {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity },
      mutations: { retry: false },
    },
  });
  client.setQueryData(["auth", "me"], mocks.user);
  render(
    <QueryClientProvider client={client}>
      <VoiceCatalog filters={filters} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
  mocks.delete.mockReset();
  mocks.patch.mockReset();
});

describe("VoiceCatalog", () => {
  it("lists voices with status as text and usage", async () => {
    mocks.get.mockResolvedValue(page([used, unused, retired]));
    mount();
    expect(
      await screen.findByRole("heading", { level: 1, name: "Voice Profiles" }),
    ).toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledWith(
      "/admin/voices",
      expect.objectContaining({
        params: { status: "all", limit: 20, offset: 0 },
      }),
    );
    const ava = await screen.findByRole("row", { name: /Ava/ });
    expect(within(ava).getByText("Active")).toBeInTheDocument();
    expect(
      within(ava).getByText("2 interviews and 1 job posting"),
    ).toBeInTheDocument();
    expect(
      within(screen.getByRole("row", { name: /Ben/ })).getByText("Not used"),
    ).toBeInTheDocument();
    expect(
      within(screen.getByRole("row", { name: /Cara/ })).getByText("Inactive"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Reactivate Cara" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Deactivate Cara" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Reactivate Ava" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Showing 1–3 of 3")).toBeInTheDocument();
  });

  it("keeps Fetch from provider disabled with the reason", async () => {
    mocks.get.mockResolvedValue(page([]));
    mount();
    const fetch = await screen.findByRole("button", {
      name: /Fetch from provider/,
    });
    expect(fetch).toBeDisabled();
    expect(fetch).toHaveAccessibleDescription(
      "Available once provider sync is connected.",
    );
  });

  it("filters through URL links and the search form", async () => {
    mocks.get.mockResolvedValue(page([used]));
    mount({ status: "all", q: "", page: 1 });
    expect(
      await screen.findByRole("link", { name: "Inactive" }),
    ).toHaveAttribute("href", "/admin/voices?status=inactive");
    fireEvent.change(screen.getByLabelText("Search by name"), {
      target: { value: "  Ava  " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Search" }));
    expect(mocks.push).toHaveBeenCalledWith("/admin/voices?q=Ava", {
      scroll: false,
    });
  });

  it("deactivates a voice in use after explaining it", async () => {
    mocks.get.mockResolvedValue(page([used]));
    mocks.delete.mockResolvedValue({
      data: {
        success: true,
        data: { outcome: "deactivated", voice: { ...used, is_active: false } },
      },
    });
    mount();
    fireEvent.click(
      await screen.findByRole("button", { name: "Deactivate Ava" }),
    );
    const dialog = await screen.findByRole("dialog", {
      name: "Deactivate this voice?",
    });
    expect(dialog).toHaveTextContent(
      "is used by 2 interviews and 1 job posting",
    );
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Deactivate voice" }),
    );
    await waitFor(() =>
      expect(mocks.delete).toHaveBeenCalledWith(`/admin/voices/${used.id}`),
    );
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    expect(mocks.toast).toHaveBeenCalledWith(
      "Voice deactivated",
      expect.anything(),
    );
  });

  it("removes an unused voice permanently", async () => {
    mocks.get.mockResolvedValue(page([unused]));
    mocks.delete.mockResolvedValue({
      data: { success: true, data: { outcome: "deleted", voice: unused } },
    });
    mount();
    fireEvent.click(await screen.findByRole("button", { name: "Remove Ben" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Remove this voice?",
    });
    expect(dialog).toHaveTextContent("removed from the catalog permanently");
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Remove voice" }),
    );
    await waitFor(() =>
      expect(mocks.toast).toHaveBeenCalledWith(
        "Voice removed",
        expect.anything(),
      ),
    );
  });

  it("keeps the dialog open with the server message when removing fails", async () => {
    mocks.get.mockResolvedValue(page([unused]));
    mocks.delete.mockRejectedValue(
      failure(500, "INTERNAL_ERROR", "voice catalog persistence failed"),
    );
    mount();
    fireEvent.click(await screen.findByRole("button", { name: "Remove Ben" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Remove voice" }),
    );
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "voice catalog persistence failed",
    );
  });

  it("refreshes when the voice was already removed", async () => {
    mocks.get.mockResolvedValueOnce(page([unused])).mockResolvedValue(page([]));
    mocks.delete.mockRejectedValue(
      failure(404, "VOICE_NOT_FOUND", "voice not found"),
    );
    mount();
    fireEvent.click(await screen.findByRole("button", { name: "Remove Ben" }));
    fireEvent.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Remove voice",
      }),
    );
    expect(await screen.findByText(/already removed/)).toBeInTheDocument();
    expect(
      await screen.findByText("There are no voices in the catalog yet."),
    ).toBeInTheDocument();
  });

  it("pages through the catalog with URL links", async () => {
    mocks.get.mockResolvedValue({
      data: {
        success: true,
        data: {
          items: [used],
          pagination: { limit: 20, offset: 20, total: 45 },
        },
      },
    });
    mount({ status: "active", q: "", page: 2 });
    expect(await screen.findByText("Showing 21–40 of 45")).toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledWith(
      "/admin/voices",
      expect.objectContaining({
        params: { status: "active", limit: 20, offset: 20 },
      }),
    );
    expect(screen.getByRole("link", { name: "Previous" })).toHaveAttribute(
      "href",
      "/admin/voices?status=active",
    );
    expect(screen.getByRole("link", { name: "Next" })).toHaveAttribute(
      "href",
      "/admin/voices?status=active&page=3",
    );
  });

  it("offers a way back from a page past the end", async () => {
    mocks.get.mockResolvedValue(page([], 3));
    mount({ status: "all", q: "", page: 4 });
    expect(
      await screen.findByText("There are no voices on this page."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Go to the first page" }),
    ).toHaveAttribute("href", "/admin/voices");
  });

  it("shows a loading state", () => {
    mocks.get.mockReturnValue(new Promise(() => {}));
    mount();
    expect(screen.getByRole("status")).toHaveTextContent(
      "Loading voice profiles…",
    );
  });

  it("closes the dialog when access is lost", async () => {
    mocks.get
      .mockResolvedValueOnce(page([unused]))
      .mockRejectedValue(failure(403, "FORBIDDEN", "forbidden"));
    mocks.delete.mockRejectedValue(failure(403, "FORBIDDEN", "forbidden"));
    mount();
    fireEvent.click(await screen.findByRole("button", { name: "Remove Ben" }));
    fireEvent.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Remove voice",
      }),
    );
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    expect(
      await screen.findByRole("heading", {
        name: /don't have access to voice profiles/,
      }),
    ).toBeInTheDocument();
  });

  it("shows the empty state for a search", async () => {
    mocks.get.mockResolvedValue(page([]));
    mount({ status: "all", q: "zzz", page: 1 });
    expect(
      await screen.findByText("No voices match “zzz”."),
    ).toBeInTheDocument();
  });

  it("retries after a load error", async () => {
    mocks.get
      .mockRejectedValueOnce(failure(500, "INTERNAL_ERROR", "boom"))
      .mockResolvedValue(page([used]));
    mount();
    fireEvent.click(await screen.findByRole("button", { name: "Try again" }));
    expect(await screen.findByText("Ava")).toBeInTheDocument();
  });

  it("shows an access message for a non-admin", async () => {
    mocks.get.mockRejectedValue(failure(403, "FORBIDDEN", "forbidden"));
    mount();
    expect(
      await screen.findByRole("heading", {
        name: /don't have access to voice profiles/,
      }),
    ).toBeInTheDocument();
  });

  it("reactivates an inactive voice", async () => {
    mocks.get.mockResolvedValue(page([retired]));
    mocks.patch.mockResolvedValue({
      data: { success: true, data: { ...retired, is_active: true } },
    });
    mount();
    fireEvent.click(
      await screen.findByRole("button", { name: "Reactivate Cara" }),
    );
    await waitFor(() =>
      expect(mocks.patch).toHaveBeenCalledWith(
        `/admin/voices/${retired.id}/activate`,
      ),
    );
    await waitFor(() =>
      expect(mocks.toast).toHaveBeenCalledWith(
        "Voice reactivated",
        expect.anything(),
      ),
    );
    expect(mocks.get).toHaveBeenCalledTimes(2);
  });

  it("offers both reactivate and remove for an inactive voice nobody uses", async () => {
    mocks.get.mockResolvedValue(page([{ ...unused, is_active: false }]));
    mount();
    expect(
      await screen.findByRole("button", { name: "Reactivate Ben" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Remove Ben" }),
    ).toBeInTheDocument();
  });

  it("lets the admin retry a failed reactivation", async () => {
    mocks.get.mockResolvedValue(page([retired]));
    mocks.patch
      .mockRejectedValueOnce(
        failure(500, "INTERNAL_ERROR", "voice catalog persistence failed"),
      )
      .mockResolvedValue({
        data: { success: true, data: { ...retired, is_active: true } },
      });
    mount();
    fireEvent.click(
      await screen.findByRole("button", { name: "Reactivate Cara" }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "voice catalog persistence failed",
    );
    fireEvent.click(screen.getByRole("button", { name: "Reactivate Cara" }));
    await waitFor(() =>
      expect(mocks.toast).toHaveBeenCalledWith(
        "Voice reactivated",
        expect.anything(),
      ),
    );
    expect(mocks.patch).toHaveBeenCalledTimes(2);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("refreshes when the voice to reactivate was deleted", async () => {
    mocks.get
      .mockResolvedValueOnce(page([retired]))
      .mockResolvedValue(page([]));
    mocks.patch.mockRejectedValue(
      failure(404, "VOICE_NOT_FOUND", "voice not found"),
    );
    mount();
    fireEvent.click(
      await screen.findByRole("button", { name: "Reactivate Cara" }),
    );
    expect(
      await screen.findByText(
        /removed permanently, so it can't be reactivated/,
      ),
    ).toBeInTheDocument();
    expect(
      await screen.findByText("There are no voices in the catalog yet."),
    ).toBeInTheDocument();
  });

  it("shows the access message when reactivating is denied", async () => {
    mocks.get
      .mockResolvedValueOnce(page([retired]))
      .mockRejectedValue(failure(403, "FORBIDDEN", "forbidden"));
    mocks.patch.mockRejectedValue(failure(403, "FORBIDDEN", "forbidden"));
    mount();
    fireEvent.click(
      await screen.findByRole("button", { name: "Reactivate Cara" }),
    );
    expect(
      await screen.findByRole("heading", {
        name: /don't have access to voice profiles/,
      }),
    ).toBeInTheDocument();
  });
});
