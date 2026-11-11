import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { InterviewGuidelines } from "@/features/admin/guidelines/components/interview-guidelines";
import { renderWithIntl } from "@tests/helpers/intl";
import type { AppLocale } from "@/lib/i18n/config";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  put: vi.fn(),
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
  apiClient: { get: mocks.get, put: mocks.put },
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));

const defaults = {
  tone: "neutral",
  question_length: "standard",
  follow_up_max: null as number | null,
  follow_up_focus: "standard",
  avoid_topics: [] as string[],
};
const settings = (
  over: Partial<{
    version: number;
    values: typeof defaults;
    ceiling: number;
    updated_at: string | null;
    updated_by_name: string;
  }> = {},
) => ({
  data: {
    success: true,
    data: {
      version: over.version ?? 0,
      values: over.values ?? defaults,
      defaults,
      allowed: {},
      effective_follow_up_ceiling: over.ceiling ?? 2,
      updated_at: over.updated_at ?? null,
      updated_by_name: over.updated_by_name ?? "",
    },
  },
});
function failure(status: number, code: string, message = "message") {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: { success: false, error: { code, message } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}
function mount(locale: AppLocale = "en") {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity },
      mutations: { retry: false },
    },
  });
  client.setQueryData(["auth", "me"], mocks.user);
  renderWithIntl(
    <QueryClientProvider client={client}>
      <InterviewGuidelines />
    </QueryClientProvider>,
    { locale },
  );
}
const radio = (name: RegExp) => screen.getByRole("radio", { name });
const saveButton = () =>
  screen.getByRole("button", { name: /Save changes|Saving/ });

beforeEach(() => {
  mocks.get.mockReset();
  mocks.put.mockReset();
});

describe("loading and access", () => {
  it("announces loading, then shows the defaults when nothing was saved", async () => {
    mocks.get.mockResolvedValue(settings());
    mount();
    expect(screen.getByRole("status")).toHaveTextContent(
      "Loading the interview guidelines",
    );
    expect(await screen.findByText("Nothing saved yet")).toBeInTheDocument();
    expect(screen.getByText("The built-in defaults are in use.")).toBeVisible();
    expect(mocks.get).toHaveBeenCalledWith("/admin/interview-guidelines", {
      signal: expect.any(AbortSignal),
    });
    expect(radio(/^Neutral/)).toBeChecked();
    expect(radio(/^Default/)).toBeChecked();
  });

  it("shows the saved version, who saved it and the follow-up ceiling", async () => {
    mocks.get.mockResolvedValue(
      settings({
        version: 3,
        ceiling: 2,
        updated_at: "2026-11-11T06:05:55Z",
        updated_by_name: "Ada Admin",
        values: { ...defaults, tone: "formal", avoid_topics: ["compensation"] },
      }),
    );
    mount();
    expect(await screen.findByText("Version 3")).toBeInTheDocument();
    expect(screen.getByText(/Saved by Ada Admin on/)).toBeInTheDocument();
    expect(
      screen.getByText("Server limit: 2 per question"),
    ).toBeInTheDocument();
    expect(radio(/^Formal/)).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: /Salary and compensation/ }),
    ).toBeChecked();
  });

  it("says plainly which fields only steer the prompt and which the server enforces", async () => {
    mocks.get.mockResolvedValue(settings());
    mount();
    await screen.findByText("Nothing saved yet");
    expect(
      screen.queryByText(/Tone, follow-up focus and avoided topics steer/),
    ).toBeNull();
    fireEvent.click(
      screen.getByRole("button", { name: "How these settings work" }),
    );
    expect(
      await screen.findByText(/Tone, follow-up focus and avoided topics steer/),
    ).toBeVisible();
    expect(
      screen.getByText(
        /Length and follow-up limits are enforced by the server/,
      ),
    ).toBeVisible();
  });

  it("offers a retry when loading fails", async () => {
    mocks.get.mockRejectedValueOnce(failure(500, "INTERNAL_ERROR"));
    mocks.get.mockResolvedValueOnce(settings());
    mount();
    expect(
      await screen.findByText("We couldn't load the interview guidelines"),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByText("Nothing saved yet")).toBeInTheDocument();
  });

  it("shows the access message for a 403", async () => {
    mocks.get.mockRejectedValue(failure(403, "FORBIDDEN"));
    mount();
    expect(
      await screen.findByText("You don't have access to interview guidelines"),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Save/ })).toBeNull();
  });
});

describe("editing and saving", () => {
  it("keeps Save off until something changes, and Discard restores the values", async () => {
    mocks.get.mockResolvedValue(settings());
    mount();
    await screen.findByText("Nothing saved yet");
    expect(saveButton()).toBeDisabled();
    expect(screen.getByText("No unsaved changes")).toBeInTheDocument();
    fireEvent.click(radio(/^Formal/));
    expect(saveButton()).toBeEnabled();
    expect(screen.getByText("Unsaved changes")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Discard" }));
    expect(radio(/^Neutral/)).toBeChecked();
    expect(saveButton()).toBeDisabled();
  });

  it("saves against the loaded version and then shows the new one", async () => {
    mocks.get.mockResolvedValue(settings({ version: 2 }));
    mocks.put.mockResolvedValue(
      settings({
        version: 3,
        updated_at: "2026-11-11T06:10:00Z",
        updated_by_name: "Admin",
        values: {
          tone: "friendly",
          question_length: "concise",
          follow_up_max: 1,
          follow_up_focus: "thorough",
          avoid_topics: ["compensation", "personal_life"],
        },
      }),
    );
    mount();
    await screen.findByText("Version 2");
    fireEvent.click(radio(/^Friendly/));
    fireEvent.click(radio(/^Thorough/));
    fireEvent.click(radio(/^Concise/));
    fireEvent.click(radio(/^1$/));
    fireEvent.click(
      screen.getByRole("checkbox", { name: /Family, relationship/ }),
    );
    fireEvent.click(
      screen.getByRole("checkbox", { name: /Salary and compensation/ }),
    );
    fireEvent.click(saveButton());
    await waitFor(() =>
      expect(mocks.put).toHaveBeenCalledWith("/admin/interview-guidelines", {
        expected_version: 2,
        values: {
          tone: "friendly",
          question_length: "concise",
          follow_up_max: 1,
          follow_up_focus: "thorough",
          avoid_topics: ["compensation", "personal_life"],
        },
      }),
    );
    expect(
      await screen.findByText(
        "Saved as version 3. It applies to interviews that start from now on.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Version 3")).toBeInTheDocument();
    expect(saveButton()).toBeDisabled();
  });

  it("shows the pending state and locks the form while the save is in flight", async () => {
    mocks.get.mockResolvedValue(settings());
    let finish: (value: unknown) => void = () => {};
    mocks.put.mockReturnValue(
      new Promise((resolve) => {
        finish = resolve;
      }),
    );
    mount();
    await screen.findByText("Nothing saved yet");
    fireEvent.click(radio(/^Formal/));
    fireEvent.click(saveButton());
    expect(await screen.findByText("Saving…")).toBeInTheDocument();
    expect(saveButton()).toBeDisabled();
    expect(radio(/^Neutral/)).toBeDisabled();
    finish(settings({ version: 1, values: { ...defaults, tone: "formal" } }));
    expect(await screen.findByText("Version 1")).toBeInTheDocument();
  });

  it("keeps the edits and says so when saving fails for another reason", async () => {
    mocks.get.mockResolvedValue(settings());
    mocks.put.mockRejectedValue(failure(500, "INTERNAL_ERROR"));
    mount();
    await screen.findByText("Nothing saved yet");
    fireEvent.click(radio(/^Formal/));
    fireEvent.click(saveButton());
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("We couldn't save the guidelines");
    expect(radio(/^Formal/)).toBeChecked();
    expect(saveButton()).toBeEnabled();
  });

  it("reloads and explains a version conflict instead of overwriting", async () => {
    mocks.get.mockResolvedValueOnce(settings({ version: 2 }));
    mocks.put.mockRejectedValue(failure(409, "GUIDELINE_VERSION_CONFLICT"));
    mocks.get.mockResolvedValueOnce(
      settings({
        version: 4,
        updated_at: "2026-11-11T06:20:00Z",
        updated_by_name: "Grace",
        values: { ...defaults, tone: "challenging" },
      }),
    );
    mount();
    await screen.findByText("Version 2");
    fireEvent.click(radio(/^Formal/));
    fireEvent.click(saveButton());
    expect(
      await screen.findByText(
        /Someone else saved version 4 while you were editing/,
      ),
    ).toBeInTheDocument();
    // The newer version is on screen and the Admin's edit was not kept or sent as a second write.
    expect(await screen.findByText("Version 4")).toBeInTheDocument();
    expect(radio(/^Challenging/)).toBeChecked();
    expect(mocks.put).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("alert")).toHaveTextContent(/were not saved/);
  });
});

describe("follow-up maximum", () => {
  it("states the effective count and that the ceiling cannot be raised", async () => {
    mocks.get.mockResolvedValue(settings({ ceiling: 1 }));
    mount();
    await screen.findByText("Nothing saved yet");
    expect(
      screen.getByText(/at most 1 follow-up per question/),
    ).toBeInTheDocument();
    fireEvent.click(radio(/^3$/));
    expect(
      screen.getByText(/The server's ceiling is 1, so 3 still means at most 1/),
    ).toBeInTheDocument();
    fireEvent.click(radio(/^0 \(none\)/));
    expect(
      screen.getByText(/at most 0 follow-ups per question/),
    ).toBeInTheDocument();
  });
});

describe("reset to defaults", () => {
  it("saves the defaults as a new version after a confirmation", async () => {
    mocks.get.mockResolvedValue(
      settings({
        version: 5,
        values: { ...defaults, tone: "formal", follow_up_max: 0 },
      }),
    );
    mocks.put.mockResolvedValue(settings({ version: 6 }));
    mount();
    await screen.findByText("Version 5");
    fireEvent.click(screen.getByRole("button", { name: "Reset defaults" }));
    const dialog = await screen.findByRole("dialog");
    expect(mocks.put).not.toHaveBeenCalled();
    expect(dialog).toHaveTextContent(/saves a new version/);
    fireEvent.click(
      within(dialog).getByRole("button", {
        name: "Save defaults as a new version",
      }),
    );
    await waitFor(() =>
      expect(mocks.put).toHaveBeenCalledWith("/admin/interview-guidelines", {
        expected_version: 5,
        values: defaults,
      }),
    );
    expect(
      await screen.findByText(/Defaults saved as version 6/),
    ).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("changes nothing when the confirmation is declined", async () => {
    mocks.get.mockResolvedValue(settings({ version: 1 }));
    mount();
    await screen.findByText("Version 1");
    fireEvent.click(screen.getByRole("button", { name: "Reset defaults" }));
    fireEvent.click(
      await screen.findByRole("button", { name: "Keep current guidelines" }),
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(mocks.put).not.toHaveBeenCalled();
  });
});

describe("Vietnamese", () => {
  it("renders the page from the Vietnamese catalog", async () => {
    mocks.get.mockResolvedValue(settings({ version: 1 }));
    mount("vi");
    expect(
      await screen.findByRole("heading", {
        name: "Hướng dẫn hành vi phỏng vấn",
      }),
    ).toBeInTheDocument();
    expect(await screen.findByText("Phiên bản 1")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Đặt lại mặc định" }),
    ).toBeInTheDocument();
  });
});
