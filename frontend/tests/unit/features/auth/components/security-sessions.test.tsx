import {
  QueryClient,
  QueryClientProvider,
  useQuery,
} from "@tanstack/react-query";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  credentialsKeys,
  getSecuritySessions,
} from "@/features/auth/api/credentials";
import { SecuritySessions } from "@/features/auth/components/security-sessions";
const sdk = vi.hoisted(() => ({
  listSessions: vi.fn(),
  revokeSession: vi.fn(),
  revokeOtherSessions: vi.fn(),
  revokeSessions: vi.fn(),
  signOut: vi.fn(),
  replace: vi.fn(),
  refresh: vi.fn(),
  success: vi.fn(),
}));
vi.mock("@/lib/auth/client", () => ({ authClient: sdk }));
vi.mock("goey-toast", () => ({ gooeyToast: { success: sdk.success } }));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: sdk.replace, refresh: sdk.refresh }),
}));
const row = (id: string) => ({
  id,
  token: `private-${id}`,
  userId: "user-1",
  createdAt: new Date(),
  updatedAt: new Date(),
  expiresAt: new Date(Date.now() + 60000),
  ipAddress: "127.0.0.1",
  userAgent: "Mozilla/5.0 Chrome/123.0 Linux",
});
function Sessions({ currentId }: { currentId?: string }) {
  const sessions = useQuery({
    queryKey: credentialsKeys.sessions("user-1"),
    queryFn: ({ signal }) => getSecuritySessions("user-1", signal),
    retry: false,
  });
  return (
    <SecuritySessions
      sessions={sessions}
      currentId={currentId}
      onChanged={vi.fn().mockResolvedValue(undefined)}
      onAccessError={vi.fn()}
    />
  );
}
function renderSessions(currentId: string | undefined = "current") {
  renderWithIntl(
    <QueryClientProvider client={new QueryClient()}>
      <Sessions currentId={currentId} />
    </QueryClientProvider>,
  );
}
describe("security sessions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sdk.listSessions.mockResolvedValue({
      data: [row("current"), row("other")],
      error: null,
    });
    for (const fn of [
      sdk.revokeSession,
      sdk.revokeOtherSessions,
      sdk.revokeSessions,
      sdk.signOut,
    ])
      fn.mockResolvedValue({ data: { status: true }, error: null });
  });
  it("identifies current by ID, never puts tokens into HTML and confirms revocation", async () => {
    renderSessions();
    await screen.findByText("This session");
    expect(document.body.innerHTML).not.toContain("private-current");
    fireEvent.click(
      screen.getByRole("button", { name: "Sign out Chrome on Linux" }),
    );
    expect(sdk.revokeSession).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Confirm sign out" }));
    await waitFor(() =>
      expect(sdk.revokeSession).toHaveBeenCalledWith({
        token: "private-other",
      }),
    );
    expect(sdk.success).toHaveBeenCalledWith("Session signed out.");
  });
  it("still leaves the workspace when all sessions were revoked but cookie cleanup fails", async () => {
    sdk.signOut.mockResolvedValue({
      data: null,
      error: { code: "UNAUTHORIZED" },
    });
    renderSessions();
    await screen.findByText("This session");
    fireEvent.click(
      screen.getByRole("button", { name: "Sign out all sessions" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Confirm sign out" }));
    await waitFor(() => expect(sdk.replace).toHaveBeenCalledWith("/login"));
    expect(sdk.revokeSessions).toHaveBeenCalledOnce();
  });
  it("shows reauthentication instead of an empty list for a stale session", async () => {
    sdk.listSessions.mockResolvedValue({
      data: null,
      error: { code: "SESSION_NOT_FRESH" },
    });
    renderSessions();
    expect(
      await screen.findByRole("link", { name: "Sign in again" }),
    ).toHaveAttribute("href", "/login?returnTo=%2Fsettings");
    expect(
      screen.queryByRole("button", { name: "Sign out all sessions" }),
    ).not.toBeInTheDocument();
  });
});
