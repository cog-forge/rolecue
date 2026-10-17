import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import {
  QueryClient,
  QueryClientProvider,
  useQuery,
} from "@tanstack/react-query";
import type { SessionUser } from "@/features/auth/schemas/session-schema";
import { OnboardingGate } from "@/features/onboarding/components/onboarding-gate";

const session = vi.hoisted(() => {
  const user: SessionUser = {
    id: "11111111-1111-4111-8111-111111111111",
    full_name: "Alex",
    email: "alex@example.com",
    role: "candidate",
    email_verified: true,
    is_locked: false,
    image: null,
    onboarding_role_selected: false,
    onboarding_completed: false,
  };
  return { user };
});
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => session.user,
}));
vi.mock("@/features/onboarding/components/onboarding-dialog", () => ({
  OnboardingDialog: () => <div role="dialog">Complete onboarding</div>,
}));

beforeEach(() => {
  session.user = {
    id: "11111111-1111-4111-8111-111111111111",
    full_name: "Alex",
    email: "alex@example.com",
    role: "candidate",
    email_verified: true,
    is_locked: false,
    image: null,
    onboarding_role_selected: false,
    onboarding_completed: false,
  };
});

function mountWorkspace() {
  const fetchWorkspace = vi.fn().mockResolvedValue("Workspace data");
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  function Workspace() {
    const query = useQuery({
      queryKey: ["workspace"],
      queryFn: fetchWorkspace,
    });
    return <p>{query.data ?? "Loading workspace"}</p>;
  }
  const tree = () => (
    <QueryClientProvider client={client}>
      <OnboardingGate>
        <Workspace />
      </OnboardingGate>
    </QueryClientProvider>
  );
  const view = render(tree());
  return { fetchWorkspace, rerender: () => view.rerender(tree()) };
}

it.each([false, true])(
  "does not mount or fetch workspace during onboarding (role selected: %s)",
  (selected) => {
    session.user.onboarding_role_selected = selected;
    const { fetchWorkspace } = mountWorkspace();
    expect(screen.getByRole("dialog")).toBeVisible();
    expect(screen.queryByText("Loading workspace")).not.toBeInTheDocument();
    expect(fetchWorkspace).not.toHaveBeenCalled();
  },
);

it("mounts and fetches the workspace after onboarding completes", async () => {
  const { fetchWorkspace, rerender } = mountWorkspace();
  session.user = {
    ...session.user,
    onboarding_role_selected: true,
    onboarding_completed: true,
  };
  rerender();
  expect(await screen.findByText("Workspace data")).toBeVisible();
  expect(fetchWorkspace).toHaveBeenCalledTimes(1);
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
});

it("allows admins to use the workspace without onboarding", async () => {
  session.user.role = "admin";
  const { fetchWorkspace } = mountWorkspace();
  await waitFor(() => expect(fetchWorkspace).toHaveBeenCalledTimes(1));
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
});
