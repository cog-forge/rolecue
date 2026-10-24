import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { CvLink } from "@/features/recruiter/applications/components/cv-link";

const mocks = vi.hoisted(() => ({
  user: {
    id: "11111111-1111-4111-8111-111111111111",
    full_name: "Rec",
    email: "rec@example.com",
    role: "recruiter" as const,
    email_verified: true,
    is_locked: false,
    image: null,
  },
}));
vi.mock("@/lib/api/client", () => ({ apiClient: {} }));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));

const postingId = "33333333-3333-4333-8333-333333333333";
const applicationId = "44444444-4444-4444-8444-000000000001";
const cvUrl = "https://files.example.com/cv/linh.pdf?signature=test";

function mount(cv: { url: string; expires_at: string } | null) {
  const client = new QueryClient();
  render(
    <QueryClientProvider client={client}>
      <CvLink postingId={postingId} applicationId={applicationId} cv={cv} />
    </QueryClientProvider>,
  );
  return client;
}

afterEach(() => {
  vi.useRealTimers();
});

describe("CvLink", () => {
  it("opens the CV in a new tab with a safe rel", () => {
    mount({ url: cvUrl, expires_at: "2099-01-01T00:00:00Z" });
    const link = screen.getByRole("link", { name: /open the cv/i });
    expect(link).toHaveAttribute("href", cvUrl);
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("says so when no CV is stored", () => {
    mount(null);
    expect(screen.getByText("No CV stored.")).toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });

  it("reloads the detail a minute before the link expires", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-10-24T10:00:00Z"));
    const client = new QueryClient();
    const invalidate = vi.spyOn(client, "invalidateQueries");
    render(
      <QueryClientProvider client={client}>
        <CvLink
          postingId={postingId}
          applicationId={applicationId}
          cv={{ url: cvUrl, expires_at: "2026-10-24T10:05:00Z" }}
        />
      </QueryClientProvider>,
    );

    vi.advanceTimersByTime(3 * 60 * 1000 + 59 * 1000);
    expect(invalidate).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1000);
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: [
        "recruiter-applications",
        mocks.user.id,
        "detail",
        postingId,
        applicationId,
      ],
    });
  });
});
