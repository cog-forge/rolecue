import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import { RoleGate } from "@/features/auth/components/role-gate";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import {
  SessionGate,
  useSessionUser,
} from "@/features/auth/components/session-gate";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  replace: vi.fn(),
  getSession: vi.fn(),
  toast: vi.fn(),
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mocks.replace }),
  usePathname: () => "/dashboard",
}));
vi.mock("@/lib/api/client", () => ({ apiClient: { get: mocks.get } }));
vi.mock("@/lib/auth/client", () => ({
  authClient: { getSession: mocks.getSession },
}));
vi.mock("goey-toast", () => ({ gooeyToast: { success: mocks.toast } }));

const user = {
  id: "9b9e9289-994d-4e62-9e65-034b9a907330",
  full_name: "Alex Doe",
  email: "alex@example.com",
  role: "candidate",
  email_verified: true,
  is_locked: false,
};
function Content() {
  const session = useSessionUser();
  return <p>Protected {session.role} content</p>;
}
function mount(
  role?: "candidate" | "recruiter" | "admin",
  client = new QueryClient({ defaultOptions: { queries: { gcTime: 0 } } }),
) {
  render(
    <QueryClientProvider client={client}>
      <SessionGate>
        {role ? (
          <RoleGate role={role}>
            <Content />
          </RoleGate>
        ) : (
          <Content />
        )}
      </SessionGate>
    </QueryClientProvider>,
  );
  return client;
}
beforeEach(() => {
  vi.clearAllMocks();
  window.history.replaceState(null, "", "/dashboard");
});

describe("validated workspace session", () => {
  it("keeps validated children mounted during a background session recheck", async () => {
    mocks.get.mockResolvedValueOnce({ data: { success: true, data: user } });
    const client = mount("candidate");
    const content = await screen.findByText("Protected candidate content");
    let resolve: (value: unknown) => void = () => {};
    mocks.get.mockReturnValueOnce(
      new Promise((done) => {
        resolve = done;
      }),
    );
    act(() => {
      void client.refetchQueries({ queryKey: ["auth", "me"] });
    });
    await waitFor(() => expect(mocks.get).toHaveBeenCalledTimes(2));
    expect(screen.getByText("Protected candidate content")).toBe(content);
    resolve({ data: { success: true, data: user } });
    await waitFor(() => expect(client.isFetching()).toBe(0));
  });
  it("does not expose content during session bootstrap", async () => {
    let resolve: (value: unknown) => void = () => {};
    mocks.get.mockReturnValue(
      new Promise((done) => {
        resolve = done;
      }),
    );
    mount("candidate");
    expect(screen.queryByText(/Protected/)).not.toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent(
      "Checking your session",
    );
    resolve({ data: { success: true, data: user } });
    expect(
      await screen.findByText("Protected candidate content"),
    ).toBeInTheDocument();
  });
  it.each(["recruiter", "admin"] as const)(
    "redirects %s away from the candidate entry without exposing children",
    async (role) => {
      mocks.get.mockResolvedValue({
        data: { success: true, data: { ...user, role } },
      });
      mount("candidate");
      await waitFor(() =>
        expect(mocks.replace).toHaveBeenCalledWith(`/${role}/dashboard`),
      );
      expect(screen.queryByText(/Protected/)).not.toBeInTheDocument();
    },
  );
  it("allows a shared account shell to consume the actual recruiter role", async () => {
    mocks.get.mockResolvedValue({
      data: { success: true, data: { ...user, role: "recruiter" } },
    });
    mount();
    expect(
      await screen.findByText("Protected recruiter content"),
    ).toBeInTheDocument();
  });
  it("does not display an old successful result during a failed recheck", async () => {
    const client = new QueryClient();
    client.setQueryData(["auth", "me"], user);
    const error = new AxiosError("expired");
    error.response = {
      status: 401,
      data: {},
      statusText: "Unauthorized",
      headers: {},
      config: { headers: new AxiosHeaders() },
    };
    mocks.get.mockRejectedValue(error);
    mount("candidate", client);
    expect(screen.queryByText(/Protected/)).not.toBeInTheDocument();
    await waitFor(() => expect(mocks.replace).toHaveBeenCalledWith("/login"));
    expect(screen.queryByText(/Protected/)).not.toBeInTheDocument();
  });
  it("fails closed for an unsupported backend role", async () => {
    mocks.get.mockResolvedValue({
      data: { success: true, data: { ...user, role: "owner" } },
    });
    mount();
    expect(
      await screen.findByRole("alert", {}, { timeout: 3000 }),
    ).toHaveTextContent("Unable to check your session");
    expect(screen.queryByText(/Protected/)).not.toBeInTheDocument();
  });
  it("cleans up the auth event before role-resolved navigation", async () => {
    window.history.replaceState(null, "", "/dashboard?auth=signed-in");
    mocks.get.mockResolvedValue({
      data: { success: true, data: { ...user, role: "admin" } },
    });
    mount("candidate");
    await waitFor(() => expect(mocks.toast).toHaveBeenCalledTimes(1));
    expect(window.location.search).toBe("");
  });
});
