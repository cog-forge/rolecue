import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AuthenticatedShell } from "@/features/auth/components/authenticated-shell";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  user: {
    id: "11111111-1111-4111-8111-111111111111",
    full_name: "Admin",
    email: "admin@example.com",
    role: "admin" as "admin" | "candidate" | "recruiter",
    email_verified: true,
    is_locked: false,
    image: null,
  },
}));
vi.mock("@/lib/api/client", () => ({ apiClient: { get: mocks.get } }));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));
vi.mock("@/features/auth/hooks/use-sign-out", () => ({
  useSignOut: () => ({ signOut: vi.fn(), pending: false, error: false }),
}));
vi.mock("@/features/billing/components/wallet-purchase", () => ({
  WalletPurchase: () => <div>Wallet</div>,
}));
vi.mock("@/components/layout/application-shell", () => ({
  ApplicationShell: ({
    children,
    navigationBadges,
  }: {
    children: ReactNode;
    navigationBadges?: Readonly<
      Record<string, { count: number; label: string }>
    >;
  }) => (
    <main>
      {children}
      {Object.entries(navigationBadges ?? {}).map(([path, badge]) => (
        <output key={path} aria-label={badge.label} data-path={path}>
          {badge.count}
        </output>
      ))}
    </main>
  ),
}));

const queue = (total: number) => ({
  data: {
    success: true,
    data: {
      items: [
        {
          id: "33333333-3333-4333-8333-333333333333",
          title: "Backend Engineer",
          status: "pending",
          recruiter: {
            id: "22222222-2222-4222-8222-222222222222",
            name: "Recruiter",
            email: "rec@example.com",
          },
          created_at: "2026-10-29T08:00:00Z",
        },
      ],
      pagination: { limit: 1, offset: 0, total },
    },
  },
});

function mount() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  render(
    <QueryClientProvider client={client}>
      <AuthenticatedShell>
        <p>Workspace content</p>
      </AuthenticatedShell>
    </QueryClientProvider>,
  );
  return client;
}

beforeEach(() => {
  mocks.get.mockReset();
  mocks.user.role = "admin";
});

describe("AuthenticatedShell moderation badge", () => {
  it("uses the pending queue total rather than the count of the one fetched row", async () => {
    mocks.get.mockResolvedValue(queue(127));
    mount();

    const count = await screen.findByLabelText(
      "job postings waiting for review",
    );
    expect(count).toHaveTextContent("127");
    expect(count).toHaveAttribute("data-path", "/admin/job-postings");
    expect(mocks.get).toHaveBeenCalledWith("/admin/job-postings", {
      params: { status: "pending", limit: 1, offset: 0 },
      signal: expect.any(AbortSignal),
    });
  });

  it("shows the new-today total on the candidate Job Board entry", async () => {
    mocks.user.role = "candidate";
    mocks.get.mockResolvedValue({
      data: {
        success: true,
        data: {
          items: [],
          pagination: { limit: 1, offset: 0, total: 3 },
        },
      },
    });
    mount();

    const count = await screen.findByLabelText("job postings new today");
    expect(count).toHaveTextContent("3");
    expect(count).toHaveAttribute("data-path", "/jobs");
    expect(mocks.get).not.toHaveBeenCalledWith(
      "/admin/job-postings",
      expect.anything(),
    );
  });

  it("never requests the admin queue or job-board count for a recruiter", async () => {
    mocks.user.role = "recruiter";
    mount();
    await act(async () => {});

    expect(screen.getByText("Workspace content")).toBeInTheDocument();
    expect(mocks.get).not.toHaveBeenCalled();
  });

  it("keeps the workspace usable and omits the badge when the queue fails", async () => {
    mocks.get.mockRejectedValue(new Error("temporarily unavailable"));
    const client = mount();
    await waitFor(() => expect(client.isFetching()).toBe(0));

    expect(screen.getByText("Workspace content")).toBeInTheDocument();
    expect(
      screen.queryByLabelText("job postings waiting for review"),
    ).not.toBeInTheDocument();
  });

  it("refreshes the pending total when a moderation decision invalidates queue lists", async () => {
    mocks.get.mockResolvedValueOnce(queue(17)).mockResolvedValue(queue(16));
    const client = mount();
    expect(
      await screen.findByLabelText("job postings waiting for review"),
    ).toHaveTextContent("17");

    await act(async () => {
      await client.invalidateQueries({
        queryKey: ["admin-moderation", mocks.user.id, "list"],
      });
    });

    await waitFor(() =>
      expect(
        screen.getByLabelText("job postings waiting for review"),
      ).toHaveTextContent("16"),
    );
    expect(mocks.get).toHaveBeenCalledTimes(2);
  });
});
