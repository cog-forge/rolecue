import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import {
  QueryClient,
  QueryClientProvider,
  QueryObserver,
} from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { ProfileOverview } from "@/features/profile/components/profile-overview";
import { profileKeys } from "@/features/profile/api/profile";
import type { Profile } from "@/features/profile/schemas/profile-schema";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  patch: vi.fn(),
  user: {
    id: "11111111-1111-4111-8111-111111111111",
    full_name: "Nam",
    email: "nam@example.com",
    role: "candidate" as "candidate" | "recruiter" | "admin",
    email_verified: true,
    is_locked: false,
    onboarding_role_selected: true,
    onboarding_completed: true,
    image: null as string | null,
  },
}));
vi.mock("@/lib/api/client", () => ({
  apiClient: { get: mocks.get, patch: mocks.patch },
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));
const profile: Profile = {
  ...mocks.user,
  company_name: null,
  company_website: null,
  created_at: "2026-10-07T16:00:00Z",
  updated_at: "2026-10-07T16:00:00Z",
};
const envelope = (p: Profile) => ({ data: { success: true, data: p } });
function mount() {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity },
      mutations: { retry: false },
    },
  });
  client.setQueryData(["auth", "me"], mocks.user);
  render(
    <QueryClientProvider client={client}>
      <ProfileOverview />
    </QueryClientProvider>,
  );
  return client;
}
function failure(status: number) {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: {
      error: {
        code: status === 401 ? "INVALID_TOKEN" : "INTERNAL_ERROR",
        message: "Save failed",
      },
    },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}
beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
  mocks.patch.mockReset();
  mocks.user.role = "candidate";
  mocks.get.mockResolvedValue(envelope(profile));
});
describe("own profile screen", () => {
  it("recovers from a rejected save after fresh session and profile checks succeed", async () => {
    mocks.patch.mockRejectedValueOnce(failure(403));
    const client = mount();
    const name = await screen.findByLabelText("Display name");
    const fresh = {
      ...profile,
      role: "recruiter" as const,
      company_name: "Acme",
    };
    const checkSession = vi.fn(async () => ({
      ...mocks.user,
      role: "recruiter" as const,
    }));
    const observer = new QueryObserver(client, {
      queryKey: ["auth", "me"],
      queryFn: checkSession,
      staleTime: Infinity,
    });
    const unsubscribe = observer.subscribe(() => {});
    try {
      mocks.get.mockResolvedValue(envelope(fresh));
      fireEvent.change(name, { target: { value: "Rejected draft" } });
      await waitFor(() =>
        expect(
          screen.getByRole("button", { name: "Save changes" }),
        ).toBeEnabled(),
      );
      fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
      expect(await screen.findByLabelText("Company name")).toHaveValue("Acme");
      expect(screen.getByLabelText("Display name")).toHaveValue(
        profile.full_name,
      );
      expect(checkSession).toHaveBeenCalledTimes(1);
      expect(mocks.patch).toHaveBeenCalledTimes(1);
      expect(
        screen.queryByText(/Checking your account access/),
      ).not.toBeInTheDocument();
    } finally {
      unsubscribe();
    }
  });
  it("loads account details and edits only personal fields for candidates", async () => {
    mount();
    expect(await screen.findByLabelText("Display name")).toHaveValue("Nam");
    expect(screen.queryByLabelText("Company name")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Role")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
  });
  it("keeps denied content hidden when a recovery read fails and supports retry", async () => {
    mocks.patch.mockRejectedValueOnce(failure(403));
    const client = mount();
    const name = await screen.findByLabelText("Display name");
    const observer = new QueryObserver(client, {
      queryKey: ["auth", "me"],
      queryFn: async () => mocks.user,
      staleTime: Infinity,
    });
    const unsubscribe = observer.subscribe(() => {});
    try {
      mocks.get
        .mockRejectedValueOnce(failure(500))
        .mockResolvedValue(envelope(profile));
      fireEvent.change(name, { target: { value: "Rejected draft" } });
      await waitFor(() =>
        expect(
          screen.getByRole("button", { name: "Save changes" }),
        ).toBeEnabled(),
      );
      fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
      await waitFor(() => expect(mocks.get).toHaveBeenCalledTimes(2));
      await waitFor(() =>
        expect(screen.getByRole("button", { name: "Retry" })).toBeEnabled(),
      );
      expect(screen.queryByLabelText("Display name")).not.toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: "Retry" }));
      expect(await screen.findByLabelText("Display name")).toHaveValue(
        profile.full_name,
      );
      expect(mocks.patch).toHaveBeenCalledTimes(1);
    } finally {
      unsubscribe();
    }
  });
  it("retries a failed initial load without rendering a fake form", async () => {
    mocks.get.mockRejectedValueOnce(new Error("offline"));
    mount();
    expect(await screen.findByRole("button", { name: "Retry" })).toBeVisible();
    expect(screen.queryByLabelText("Display name")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByLabelText("Display name")).toHaveValue("Nam");
  });
  it("saves normalized values and updates the session display", async () => {
    const saved = { ...profile, full_name: "New name" };
    mocks.patch.mockResolvedValue(envelope(saved));
    const client = mount();
    fireEvent.change(await screen.findByLabelText("Display name"), {
      target: { value: "  New name  " },
    });
    const save = screen.getByRole("button", { name: "Save changes" });
    await waitFor(() => expect(save).toBeEnabled());
    fireEvent.click(save);
    await waitFor(() =>
      expect(screen.getByRole("status")).toHaveTextContent(
        "Your profile has been saved",
      ),
    );
    expect(mocks.patch).toHaveBeenCalledWith("/profile", {
      full_name: "New name",
    });
    expect(screen.getByLabelText("Display name")).toHaveValue("New name");
    expect(client.getQueryData(["auth", "me"])).toMatchObject({
      full_name: "New name",
    });
    expect(save).toBeDisabled();
  });
  it("retains a failed save draft and offers discard", async () => {
    mocks.patch.mockRejectedValue(failure(500));
    mount();
    fireEvent.change(await screen.findByLabelText("Display name"), {
      target: { value: "Draft" },
    });
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Save changes" }),
      ).toBeEnabled(),
    );
    fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Save failed");
    expect(screen.getByLabelText("Display name")).toHaveValue("Draft");
    fireEvent.click(screen.getByRole("button", { name: "Discard" }));
    expect(screen.getByLabelText("Display name")).toHaveValue("Nam");
  });
  it("keeps dirty fields after a background refetch, even after an earlier successful save", async () => {
    mocks.patch.mockResolvedValue(envelope({ ...profile, full_name: "Saved" }));
    const client = mount();
    const name = await screen.findByLabelText("Display name");
    fireEvent.change(name, { target: { value: "Saved" } });
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Save changes" }),
      ).toBeEnabled(),
    );
    fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() =>
      expect(screen.getByRole("status")).toHaveTextContent(
        "Your profile has been saved",
      ),
    );
    const nextName = screen.getByLabelText("Display name");
    await act(async () => {
      fireEvent.change(nextName, { target: { value: "Next draft" } });
    });
    await waitFor(() =>
      expect(screen.getByRole("status")).toHaveTextContent(
        "You have unsaved changes",
      ),
    );
    await act(async () => {
      client.setQueryData(profileKeys.me(profile.id), {
        ...profile,
        full_name: "Elsewhere",
      });
    });
    expect(nextName).toHaveValue("Next draft");
  });
  it("keeps a dirty draft when a background refresh fails", async () => {
    const client = mount();
    const name = await screen.findByLabelText("Display name");
    fireEvent.change(name, { target: { value: "Draft" } });
    mocks.get.mockRejectedValue(new Error("offline"));
    await act(async () => {
      await client.refetchQueries({ queryKey: profileKeys.me(profile.id) });
    });
    expect(name).toHaveValue("Draft");
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Unable to refresh",
    );
  });
  it("exposes recruiter metadata and validates invalid HTTPS links", async () => {
    mocks.user.role = "recruiter";
    mocks.get.mockResolvedValue(
      envelope({ ...profile, role: "recruiter", company_name: "Acme" }),
    );
    mount();
    expect(await screen.findByLabelText("Company name")).toHaveValue("Acme");
    fireEvent.change(screen.getByLabelText("Company website"), {
      target: { value: "http://acme.example" },
    });
    expect(await screen.findByRole("alert")).toHaveTextContent("HTTPS website");
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
  });
  it("hides the form on denied mutation and revalidates the existing auth gate", async () => {
    mocks.patch.mockRejectedValue(failure(401));
    const client = mount();
    const invalidation = vi.spyOn(client, "invalidateQueries");
    fireEvent.change(await screen.findByLabelText("Display name"), {
      target: { value: "Draft" },
    });
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Save changes" }),
      ).toBeEnabled(),
    );
    fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() =>
      expect(screen.queryByLabelText("Display name")).not.toBeInTheDocument(),
    );
    expect(invalidation).toHaveBeenCalledWith({ queryKey: ["auth", "me"] });
    expect(screen.getByRole("status")).toHaveTextContent(
      "We couldn't confirm your account access",
    );
    expect(mocks.get).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("button", { name: "Retry" })).toBeEnabled();
  });
});
