import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { VoiceCatalog } from "@/features/admin/voices/components/voice-catalog";
import { renderWithIntl } from "@tests/helpers/intl";
import {
  defaultVoiceFilters,
  type VoiceFilters,
} from "@/features/admin/voices/utils/voice-filters";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  delete: vi.fn(),
  patch: vi.fn(),
  post: vi.fn(),
  push: vi.fn(),
  desktop: true,
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
  apiClient: {
    get: mocks.get,
    delete: mocks.delete,
    patch: mocks.patch,
    post: mocks.post,
  },
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
  provider: "azure",
  provider_voice_id: "en-US-AvaNeural",
  locale: "en-US",
  gender: "Female",
  interview_count: 2,
  job_posting_count: 1,
  in_use: true,
};
const unused = {
  ...used,
  id: "33333333-3333-4333-8333-333333333332",
  name: "Ben",
  provider: null,
  provider_voice_id: null,
  locale: null,
  gender: null,
  interview_count: 0,
  job_posting_count: 0,
  in_use: false,
};
const retired = {
  ...used,
  id: "33333333-3333-4333-8333-333333333333",
  name: "Cara",
  provider_voice_id: "en-US-CaraNeural",
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
  renderWithIntl(
    <QueryClientProvider client={client}>
      <VoiceCatalog filters={filters} />
    </QueryClientProvider>,
  );
}

// The details sit beside the grid from the lg breakpoint; below it they open in a sheet.
function stubViewport() {
  vi.stubGlobal("matchMedia", (media: string) => ({
    matches: mocks.desktop,
    media,
    addEventListener: () => {},
    removeEventListener: () => {},
  }));
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.desktop = true;
  stubViewport();
  mocks.get.mockReset();
  mocks.delete.mockReset();
  mocks.patch.mockReset();
  mocks.post.mockReset();
});

describe("VoiceCatalog", () => {
  const card = async (name: string) =>
    within(await screen.findByRole("list", { name: "Voices" })).getByRole(
      "button",
      { name: new RegExp(`^${name}`) },
    );

  it("shows each voice as a card with its locale, status as text and usage", async () => {
    mocks.get.mockResolvedValue(page([used, unused, retired]));
    mount();
    expect(
      await screen.findByRole("heading", { level: 1, name: "Voice Profiles" }),
    ).toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledWith(
      "/admin/voices",
      expect.objectContaining({
        params: { status: "all", limit: 9, offset: 0 },
      }),
    );
    const ava = await card("Ava");
    expect(ava).toHaveTextContent("Female");
    expect(ava).toHaveTextContent("en-US");
    expect(ava).toHaveTextContent("Active");
    expect(ava).toHaveTextContent("2 interviews and 1 job posting");
    // The provider voice ID is a detail, not part of the card.
    expect(ava).not.toHaveTextContent("en-US-AvaNeural");
    const ben = await card("Ben");
    expect(ben).toHaveTextContent("Gender not set");
    expect(ben).not.toHaveTextContent("en-");
    expect(await card("Cara")).toHaveTextContent("Inactive");
    expect(screen.getByText("Showing 1–3 of 3")).toBeInTheDocument();
  });

  it("selects the first voice beside the grid and shows its details and actions", async () => {
    mocks.get.mockResolvedValue(page([used, unused, retired]));
    mount();
    const details = await screen.findByRole("complementary", {
      name: "Voice details",
    });
    expect(
      within(details).getByRole("heading", { name: "Ava" }),
    ).toBeInTheDocument();
    expect(
      within(details).getByText("English (United States)"),
    ).toBeInTheDocument();
    expect(within(details).getByText("azure")).toBeInTheDocument();
    expect(within(details).getByText("en-US-AvaNeural")).toBeInTheDocument();
    expect(
      within(details).getByRole("button", { name: "Deactivate Ava" }),
    ).toBeInTheDocument();
    expect(await card("Ava")).toHaveAttribute("aria-pressed", "true");

    // A voice nobody uses can be removed; an inactive one can be reactivated.
    fireEvent.click(await card("Ben"));
    expect(await card("Ben")).toHaveAttribute("aria-pressed", "true");
    expect(await card("Ava")).toHaveAttribute("aria-pressed", "false");
    await within(details).findByRole("heading", { name: "Ben" });
    // A legacy voice has no provider metadata: gender, locale, provider and ID all read "Not set".
    expect(within(details).getAllByText("Not set")).toHaveLength(4);
    expect(
      within(details).getByRole("button", { name: "Remove Ben" }),
    ).toBeInTheDocument();
    fireEvent.click(await card("Cara"));
    await within(details).findByRole("heading", { name: "Cara" });
    expect(
      within(details).getByRole("button", { name: "Reactivate Cara" }),
    ).toBeInTheDocument();
    // Cara is inactive and still in use, so only reactivating is offered.
    expect(
      within(details).queryByRole("button", { name: "Deactivate Cara" }),
    ).not.toBeInTheDocument();
    expect(
      within(details).queryByRole("button", { name: "Reactivate Ava" }),
    ).not.toBeInTheDocument();
  });

  it("opens the details in a sheet on small screens", async () => {
    mocks.desktop = false;
    mocks.get.mockResolvedValue(page([used, unused]));
    mount();
    expect(screen.queryByRole("complementary")).not.toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    fireEvent.click(await card("Ben"));
    const sheet = await screen.findByRole("dialog");
    expect(
      await within(sheet).findByRole("heading", { name: "Ben" }),
    ).toBeInTheDocument();
    expect(
      within(sheet).getByRole("button", { name: "Remove Ben" }),
    ).toBeInTheDocument();
  });

  describe("sync from provider", () => {
    const summary = {
      provider: "azure",
      fetched: 6,
      added: 3,
      updated: 1,
      deactivated: 1,
      unchanged: 1,
    };
    const synced = { data: { success: true, data: summary } };

    it("syncs, shows progress and the result summary, and refreshes the list", async () => {
      let finish: (value: typeof synced) => void = () => {};
      mocks.post.mockReturnValue(
        new Promise((resolve) => {
          finish = resolve;
        }),
      );
      mocks.get.mockResolvedValue(page([used]));
      mount();
      const button = await screen.findByRole("button", {
        name: "Sync from provider",
      });
      fireEvent.click(button);

      const syncing = await screen.findByRole("button", { name: "Syncing…" });
      expect(syncing).toBeDisabled();
      expect(
        screen.getByText("Syncing voices from the provider…"),
      ).toBeInTheDocument();
      expect(mocks.post).toHaveBeenCalledTimes(1);
      expect(mocks.post).toHaveBeenCalledWith("/admin/voices/sync");

      finish(synced);
      await screen.findByText(/Synced 6 voices from azure/);
      const result = screen
        .getByText(/Synced 6 voices from azure/)
        .closest('[role="status"]') as HTMLElement;
      for (const [label, value] of [
        ["Added", "3"],
        ["Updated", "1"],
        ["Deactivated", "1"],
        ["Unchanged", "1"],
      ])
        expect(
          within(result).getByText(label).nextElementSibling,
        ).toHaveTextContent(value);
      expect(
        screen.getByRole("button", { name: "Sync from provider" }),
      ).toBeEnabled();
      await waitFor(() => expect(mocks.get).toHaveBeenCalledTimes(2));
      expect(mocks.toast).toHaveBeenCalledWith(
        "Voice catalog synced",
        expect.objectContaining({
          description: "3 added, 1 updated, 1 deactivated, 1 unchanged.",
        }),
      );

      fireEvent.click(screen.getByRole("button", { name: "Dismiss" }));
      await waitFor(() =>
        expect(screen.queryByText(/Synced 6 voices/)).not.toBeInTheDocument(),
      );
    });

    it("explains a provider failure, says nothing changed and retries", async () => {
      mocks.get.mockResolvedValue(page([used]));
      mocks.post
        .mockRejectedValueOnce(
          failure(
            502,
            "VOICE_SYNC_FAILED",
            "the voice provider could not be reached",
          ),
        )
        .mockResolvedValueOnce(synced);
      mount();
      fireEvent.click(
        await screen.findByRole("button", { name: "Sync from provider" }),
      );
      const alert = await screen.findByRole("alert");
      expect(alert).toHaveTextContent(
        "the voice provider could not be reached. The catalog was not changed.",
      );
      expect(mocks.toast).not.toHaveBeenCalled();

      fireEvent.click(within(alert).getByRole("button", { name: "Try again" }));
      expect(
        await screen.findByText(/Synced 6 voices from azure/),
      ).toBeInTheDocument();
      expect(
        screen.queryByText(/could not be reached/),
      ).not.toBeInTheDocument();
    });

    it("uses the API message when the provider is not configured", async () => {
      mocks.get.mockResolvedValue(page([]));
      mocks.post.mockRejectedValue(
        failure(
          503,
          "VOICE_PROVIDER_UNAVAILABLE",
          "the voice provider is not configured",
        ),
      );
      mount();
      fireEvent.click(
        await screen.findByRole("button", { name: "Sync from provider" }),
      );
      expect(await screen.findByRole("alert")).toHaveTextContent(
        "the voice provider is not configured. The catalog was not changed.",
      );
    });

    it("falls back to a generic message for unexpected failures", async () => {
      mocks.get.mockResolvedValue(page([]));
      mocks.post.mockRejectedValue(
        failure(500, "INTERNAL_ERROR", "sql: secret detail"),
      );
      mount();
      fireEvent.click(
        await screen.findByRole("button", { name: "Sync from provider" }),
      );
      const alert = await screen.findByRole("alert");
      expect(alert).toHaveTextContent(
        "We couldn't sync the voices from the provider.",
      );
      expect(alert).not.toHaveTextContent("secret detail");
    });

    it("shows the access message when the session lost admin rights", async () => {
      mocks.get.mockResolvedValue(page([used]));
      mocks.post.mockRejectedValue(failure(403, "FORBIDDEN", "forbidden"));
      mount();
      fireEvent.click(
        await screen.findByRole("button", { name: "Sync from provider" }),
      );
      expect(
        await screen.findByRole("heading", {
          name: /don't have access to voice profiles/,
        }),
      ).toBeInTheDocument();
      expect(
        screen.queryByText(/catalog was not changed/),
      ).not.toBeInTheDocument();
    });
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
          pagination: { limit: 9, offset: 9, total: 55 },
        },
      },
    });
    mount({ status: "active", q: "", page: 2 });
    expect(await screen.findByText("Showing 10–18 of 55")).toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledWith(
      "/admin/voices",
      expect.objectContaining({
        params: { status: "active", limit: 9, offset: 9 },
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
    expect(
      within(await screen.findByRole("list", { name: "Voices" })).getByText(
        "Ava",
      ),
    ).toBeInTheDocument();
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
