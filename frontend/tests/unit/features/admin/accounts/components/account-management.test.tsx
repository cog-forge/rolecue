import { z } from "zod";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import {
  QueryClient,
  QueryClientProvider,
  QueryObserver,
} from "@tanstack/react-query";
import { AccountManagement } from "@/features/admin/accounts/components/account-management";

const mocks = vi.hoisted(() => ({
  list: vi.fn(),
  lock: vi.fn(),
  unlock: vi.fn(),
  detail: vi.fn(),
  user: { id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", role: "admin" },
}));
vi.mock("@/lib/api/client", () => ({
  apiClient: {
    get: async (url: string, options: { params?: unknown }) =>
      backendResponse(
        await (url === "/admin/users"
          ? mocks.list(options)
          : mocks.detail(url)),
      ),
    post: async (url: string, body: unknown) =>
      backendResponse(
        await (url.endsWith("/lock") ? mocks.lock(body) : mocks.unlock(body)),
        true,
      ),
  },
}));

function backendResponse(value: unknown, transition = false) {
  const response = z
    .object({
      data: z.unknown(),
      error: z
        .object({ status: z.number(), message: z.string().optional() })
        .nullish(),
    })
    .parse(value);
  if (response.error)
    throw Object.assign(new Error(response.error.message), {
      isAxiosError: true,
      response: {
        status: response.error.status,
        data: { error: { message: response.error.message } },
      },
    });
  const account = z
    .object({
      id: z.string(),
      name: z.string(),
      email: z.string(),
      role: z.string(),
      banned: z.boolean(),
      createdAt: z.string(),
      banReason: z.string().nullish(),
      banExpires: z.string().nullish(),
    })
    .transform((u) => ({
      id: u.id,
      name: u.name,
      email: u.email,
      role: u.role,
      is_locked: u.banned,
      created_at: u.createdAt,
      lock_reason: u.banReason,
      lock_expires_at: u.banExpires,
      email_verified: true,
      image: null,
    }));
  if (transition)
    return {
      data: {
        success: true,
        data: {
          user: account.parse(
            z.object({ user: z.unknown() }).parse(response.data).user,
          ),
          changed: true,
          notification: { status: "accepted" },
        },
      },
    };
  const list = z
    .object({ users: z.array(account), total: z.number() })
    .safeParse(response.data);
  return {
    data: {
      success: true,
      data: list.success ? list.data : account.parse(response.data),
    },
  };
}
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));
vi.mock("goey-toast", () => ({
  gooeyToast: { success: vi.fn(), error: vi.fn() },
}));
vi.mock("@/components/account/user-avatar", () => ({
  UserAvatar: () => <span />,
}));

const candidate = {
  id: "11111111-1111-4111-8111-111111111111",
  name: "Alex Candidate",
  email: "alex@example.com",
  role: "candidate",
  banned: false,
  createdAt: "2026-10-01T12:00:00Z",
  banReason: null,
  banExpires: null,
};
const recruiter = {
  ...candidate,
  id: "22222222-2222-4222-8222-222222222222",
  name: "Sam Recruiter",
  email: "sam@example.com",
  role: "recruiter",
  banned: true,
  banReason: "Spam",
  banExpires: "2099-10-01T12:00:00Z",
};
const admin = {
  ...candidate,
  id: mocks.user.id,
  name: "Admin",
  email: "admin@example.com",
  role: "admin",
};
const result = (
  users: Array<
    Omit<typeof candidate, "banReason" | "banExpires"> & {
      banReason: string | null;
      banExpires: string | null;
    }
  > = [candidate, recruiter, admin],
  total = users.length,
) => ({ data: { users, total }, error: null });
async function chooseAccountOption(label: string, name: string) {
  fireEvent.keyDown(screen.getByRole("combobox", { name: label }), {
    key: "ArrowDown",
  });
  fireEvent.keyDown(await screen.findByRole("option", { name }), {
    key: "Enter",
  });
}

async function openAccountAction(name: string, locked = false) {
  const button = await screen.findByRole("button", {
    name: `Actions for ${name}`,
  });
  await waitFor(() => expect(button).toBeEnabled());
  fireEvent.keyDown(button, { key: "Enter" });
  fireEvent.click(
    await screen.findByRole("menuitem", {
      name: locked ? "Unlock account" : "Lock account",
    }),
  );
}

function mount() {
  const cache = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={cache}>
      <AccountManagement />
    </QueryClientProvider>,
  );
  return cache;
}
beforeEach(() => {
  if (!HTMLElement.prototype.scrollIntoView)
    Object.defineProperty(HTMLElement.prototype, "scrollIntoView", {
      configurable: true,
      value: vi.fn(),
    });
  vi.clearAllMocks();
  mocks.list.mockReset().mockResolvedValue(result());
  mocks.lock.mockReset().mockResolvedValue({
    data: { user: { ...candidate, banned: true } },
    error: null,
  });
  mocks.detail.mockReset().mockResolvedValue({
    data: {
      ...candidate,
      emailVerified: true,
      updatedAt: candidate.createdAt,
    },
    error: null,
  });
  mocks.unlock.mockReset().mockResolvedValue({
    data: { user: { ...recruiter, banned: false } },
    error: null,
  });
});

describe("administrator account list", () => {
  it("does not repeatedly retry a denied list when the auth gate still says admin", async () => {
    const forbidden = { data: null, error: { status: 403 } };
    mocks.list
      .mockResolvedValueOnce(forbidden)
      .mockResolvedValueOnce(forbidden)
      .mockImplementation(() => new Promise(() => {}));
    const cache = mount();
    const observer = new QueryObserver(cache, {
      queryKey: ["auth", "me"],
      queryFn: async () => ({ ...mocks.user, role: "admin" }),
      staleTime: Infinity,
    });
    const unsubscribe = observer.subscribe(() => {});
    try {
      await screen.findByRole("button", { name: "Retry access check" });
      await act(async () => {
        await new Promise((resolve) => setTimeout(resolve, 50));
      });
      expect(mocks.list).toHaveBeenCalledTimes(2);
      expect(screen.queryByRole("table")).not.toBeInTheDocument();
    } finally {
      unsubscribe();
    }
  });
  it("validates custom datetime and preserves the choice when toggling indefinite lock", async () => {
    mount();
    await openAccountAction("Alex Candidate");
    fireEvent.change(screen.getByLabelText("Reason sent to user"), {
      target: { value: "Abuse" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Custom date/time" }));
    fireEvent.change(screen.getByLabelText("Lock until"), {
      target: { value: "2020-01-01T12:00" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Confirm lock" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("future");
    expect(mocks.lock).not.toHaveBeenCalled();
    fireEvent.change(screen.getByLabelText("Lock until"), {
      target: { value: "2099-10-10T12:00" },
    });
    fireEvent.click(screen.getByLabelText("Lock indefinitely"));
    expect(screen.getByLabelText("Lock until")).toBeDisabled();
    fireEvent.click(screen.getByLabelText("Lock indefinitely"));
    expect(screen.getByLabelText("Lock until")).toHaveValue("2099-10-10T12:00");
    fireEvent.click(screen.getByRole("button", { name: "Confirm lock" }));
    await waitFor(() =>
      expect(mocks.lock).toHaveBeenCalledWith(
        expect.objectContaining({
          reason: "Abuse",
          expires_in: expect.any(Number),
        }),
      ),
    );
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
  });
  it("resumes after a denied operation only when a fresh gate and list check both succeed", async () => {
    mocks.lock.mockResolvedValue({
      data: null,
      error: { status: 403, message: "Permission changed" },
    });
    const cache = mount();
    const checkSession = vi.fn(async () => ({
      ...mocks.user,
      email: "admin@example.com",
      role: "admin",
    }));
    const observer = new QueryObserver(cache, {
      queryKey: ["auth", "me"],
      queryFn: checkSession,
      staleTime: Infinity,
    });
    const unsubscribe = observer.subscribe(() => {});
    try {
      await openAccountAction("Alex Candidate");
      fireEvent.change(screen.getByLabelText("Reason sent to user"), {
        target: { value: "Abuse" },
      });
      fireEvent.click(screen.getByLabelText("Lock indefinitely"));
      fireEvent.click(screen.getByRole("button", { name: "Confirm lock" }));
      await waitFor(() => expect(checkSession).toHaveBeenCalledTimes(2));
      await waitFor(() =>
        expect(
          screen.getByRole("button", { name: "Actions for Alex Candidate" }),
        ).toBeEnabled(),
      );
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
      expect(mocks.lock).toHaveBeenCalledTimes(1);
    } finally {
      unsubscribe();
    }
  });
  it("shows account state and keeps Admin and unknown roles read-only", async () => {
    mocks.list.mockResolvedValue(
      result([
        candidate,
        recruiter,
        admin,
        {
          ...candidate,
          id: "33333333-3333-4333-8333-333333333333",
          name: "Unknown user",
          role: "unexpected",
        },
      ]),
    );
    mount();
    const table = await screen.findByRole("table", {
      name: "Platform accounts",
    });
    expect(within(table).getByText("Alex Candidate")).toBeVisible();
    expect(within(table).getByText("Spam")).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Actions for Alex Candidate" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: "Actions for Sam Recruiter" }),
    ).toBeEnabled();
    expect(
      screen.queryByRole("button", { name: "Lock Admin" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Lock Unknown user" }),
    ).not.toBeInTheDocument();
  });
  it("runs name search and a role filter on the server, resetting pagination", async () => {
    mocks.list.mockResolvedValue(result([candidate], 41));
    mount();
    await screen.findByRole("table");
    fireEvent.click(screen.getByRole("button", { name: "Next page" }));
    await waitFor(() =>
      expect(mocks.list).toHaveBeenLastCalledWith(
        expect.objectContaining({
          params: expect.objectContaining({ offset: 20 }),
        }),
      ),
    );
    fireEvent.change(screen.getByLabelText("Search accounts"), {
      target: { value: "  Alex  " },
    });
    await chooseAccountOption("Filter accounts", "Recruiter");
    await waitFor(() =>
      expect(mocks.list).toHaveBeenLastCalledWith(
        expect.objectContaining({
          params: expect.objectContaining({
            offset: 0,
            search: "Alex",
            filter: "recruiter",
          }),
        }),
      ),
    );
  });
  it("renders errors with Retry and distinct no-result/empty states", async () => {
    mocks.list
      .mockResolvedValueOnce({ data: null, error: { status: 500 } })
      .mockResolvedValue(result([], 0));
    mount();
    fireEvent.click(await screen.findByRole("button", { name: "Retry" }));
    expect(await screen.findByText("No accounts yet")).toBeVisible();
    expect(screen.getByRole("button", { name: "Next page" })).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Search accounts"), {
      target: { value: "missing" },
    });
    expect(await screen.findByText("No matching accounts")).toBeVisible();
    fireEvent.click(
      screen.getByRole("button", { name: "Clear search and filter" }),
    );
    expect(await screen.findByText("No accounts yet")).toBeVisible();
  });
  it("cancel never sends a lock and indefinite lock omits duration", async () => {
    mount();
    await openAccountAction("Alex Candidate");
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(mocks.lock).not.toHaveBeenCalled();
    await openAccountAction("Alex Candidate");
    fireEvent.change(screen.getByLabelText("Reason sent to user"), {
      target: { value: "  Abuse  " },
    });
    fireEvent.click(screen.getByLabelText("Lock indefinitely"));
    expect(screen.getByRole("group", { name: "Duration" })).toBeDisabled();
    mocks.list.mockResolvedValue(
      result([
        { ...candidate, banned: true, banReason: "Abuse" },
        recruiter,
        admin,
      ]),
    );
    fireEvent.click(screen.getByRole("button", { name: "Confirm lock" }));
    await waitFor(() =>
      expect(mocks.lock).toHaveBeenCalledWith({
        reason: "Abuse",
      }),
    );
    expect(
      await screen.findByRole("button", { name: "Actions for Alex Candidate" }),
    ).toBeEnabled();
  });
  it("keeps a failed lock draft and never flips the authoritative status", async () => {
    mocks.lock.mockResolvedValue({
      data: null,
      error: { status: 400, message: "Invalid lock" },
    });
    mount();
    await openAccountAction("Alex Candidate");
    fireEvent.change(screen.getByLabelText("Reason sent to user"), {
      target: { value: "Abuse" },
    });
    fireEvent.click(screen.getByRole("button", { name: "1 hour" }));
    fireEvent.click(screen.getByRole("button", { name: "Confirm lock" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Invalid lock");
    expect(screen.getByLabelText("Reason sent to user")).toHaveValue("Abuse");
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(
      screen.getByRole("button", { name: "Actions for Alex Candidate" }),
    ).toBeInTheDocument();
  });
  it("requires explicit confirmation to unlock and refreshes the list", async () => {
    mount();
    await openAccountAction("Sam Recruiter", true);
    expect(mocks.unlock).not.toHaveBeenCalled();
    mocks.list.mockResolvedValue(
      result([
        candidate,
        { ...recruiter, banned: false, banReason: null, banExpires: null },
        admin,
      ]),
    );
    fireEvent.click(screen.getByRole("button", { name: "Confirm unlock" }));
    expect(
      await screen.findByRole("button", { name: "Actions for Sam Recruiter" }),
    ).toBeEnabled();
    expect(mocks.unlock).toHaveBeenCalledWith({});
  });
  it("blocks another mutation when a successful write cannot refresh, then retries only the read", async () => {
    mount();
    await openAccountAction("Alex Candidate");
    fireEvent.change(screen.getByLabelText("Reason sent to user"), {
      target: { value: "Abuse" },
    });
    fireEvent.click(screen.getByLabelText("Lock indefinitely"));
    mocks.list.mockResolvedValueOnce({
      data: null,
      error: { status: 500 },
    });
    fireEvent.click(screen.getByRole("button", { name: "Confirm lock" }));
    const retry = await screen.findByRole("button", { name: "Retry refresh" });
    expect(
      screen.getByRole("button", { name: "Actions for Alex Candidate" }),
    ).toBeDisabled();
    mocks.list.mockResolvedValue(
      result([
        { ...candidate, banned: true, banReason: "Abuse" },
        recruiter,
        admin,
      ]),
    );
    fireEvent.click(retry);
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Actions for Alex Candidate" }),
      ).toBeEnabled(),
    );
    expect(mocks.lock).toHaveBeenCalledTimes(1);
  });
  it("reconciles a lost mutation response from authoritative data without sending the write again", async () => {
    mocks.lock.mockRejectedValue(new TypeError("Network error"));
    mount();
    await openAccountAction("Alex Candidate");
    fireEvent.change(screen.getByLabelText("Reason sent to user"), {
      target: { value: "Abuse" },
    });
    fireEvent.click(screen.getByLabelText("Lock indefinitely"));
    mocks.list.mockResolvedValue(
      result([
        { ...candidate, banned: true, banReason: "Abuse" },
        recruiter,
        admin,
      ]),
    );
    fireEvent.click(screen.getByRole("button", { name: "Confirm lock" }));
    expect(
      await screen.findByRole("button", { name: "Actions for Alex Candidate" }),
    ).toBeEnabled();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(mocks.lock).toHaveBeenCalledTimes(1);
  });
  it("prevents repeated submissions while a write is pending", async () => {
    let complete: (value: unknown) => void = () => {};
    mocks.lock.mockImplementation(
      () =>
        new Promise((resolve) => {
          complete = resolve;
        }),
    );
    mount();
    await openAccountAction("Alex Candidate");
    fireEvent.change(screen.getByLabelText("Reason sent to user"), {
      target: { value: "Abuse" },
    });
    fireEvent.click(screen.getByLabelText("Lock indefinitely"));
    const confirmation = screen.getByRole("button", { name: "Confirm lock" });
    fireEvent.click(confirmation);
    fireEvent.click(confirmation);
    await waitFor(() => expect(mocks.lock).toHaveBeenCalledTimes(1));
    expect(screen.getByRole("button", { name: "Updating…" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    complete({ data: { user: { ...candidate, banned: true } }, error: null });
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
  });
  it("hides cached account records on a denied request and revalidates the auth gate", async () => {
    mocks.lock.mockResolvedValue({
      data: null,
      error: { status: 403, message: "Permission changed" },
    });
    const cache = mount();
    cache.setQueryData(["auth", "me"], { ...mocks.user, role: "admin" });
    const check = vi.spyOn(cache, "invalidateQueries");
    await openAccountAction("Alex Candidate");
    fireEvent.change(screen.getByLabelText("Reason sent to user"), {
      target: { value: "Abuse" },
    });
    fireEvent.click(screen.getByLabelText("Lock indefinitely"));
    fireEvent.click(screen.getByRole("button", { name: "Confirm lock" }));
    expect(
      await screen.findByRole("button", { name: "Retry access check" }),
    ).toBeVisible();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    await waitFor(() =>
      expect(check).toHaveBeenCalledWith({ queryKey: ["auth", "me"] }),
    );
    expect(mocks.lock).toHaveBeenCalledTimes(1);
  });
  it("returns to a valid page when the current page disappears after a refresh", async () => {
    mocks.list.mockResolvedValue(result([candidate], 21));
    mount();
    await screen.findByRole("table");
    fireEvent.click(screen.getByRole("button", { name: "Next page" }));
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Refresh" })).toBeEnabled(),
    );
    mocks.list.mockResolvedValue(result([], 20));
    fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
    await waitFor(() =>
      expect(mocks.list).toHaveBeenLastCalledWith(
        expect.objectContaining({
          params: expect.objectContaining({ offset: 0 }),
        }),
      ),
    );
  });
});
