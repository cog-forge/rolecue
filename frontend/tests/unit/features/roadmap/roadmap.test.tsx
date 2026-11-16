import { describe, expect, it, vi, beforeEach } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { renderWithIntl } from "@tests/helpers/intl";
import {
  roadmapFixture,
  profileFixture,
  profileId,
  itemId,
  sessionId,
} from "@tests/fixtures/roadmap";
import { RoadmapScreen } from "@/features/roadmap/components/roadmap-screen";
import { TopicScore } from "@/features/roadmap/components/topic-status";
import { roadmapPollInterval } from "@/features/roadmap/lib/use-roadmap";
import {
  getSessionTopicEvidence,
  roadmapSchema,
} from "@/features/roadmap/api/roadmap";
const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
vi.mock("@/lib/api/client", () => ({ apiClient: mocks }));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => ({ id: "user" }),
}));
const ok = (data: unknown) => ({ data: { success: true, data } });
function renderScreen(locale: "en" | "vi" = "en") {
  return renderWithIntl(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <RoadmapScreen profileId={profileId} />
    </QueryClientProvider>,
    { locale },
  );
}
beforeEach(() => {
  mocks.get.mockReset();
  mocks.post.mockReset();
  mocks.get.mockImplementation((path: string) =>
    Promise.resolve(
      ok(path.endsWith("/roadmap") ? roadmapFixture() : profileFixture),
    ),
  );
});
describe("Roadmap preparation and evidence", () => {
  it("shows a recoverable read error without inventing a preparation state", async () => {
    mocks.get.mockImplementation((path: string) =>
      path.endsWith("/roadmap")
        ? Promise.reject(new Error("offline"))
        : Promise.resolve(ok(profileFixture)),
    );
    renderScreen();
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Your roadmap could not be loaded",
    );
    expect(
      screen.queryByText("Your roadmap starts with your requirements"),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Try again" })).toBeEnabled();
  });
  it.each(["en", "vi"] as const)(
    "renders progress and freely selects a later topic in %s",
    async (locale) => {
      renderScreen(locale);
      expect(
        await screen.findByText(
          locale === "en" ? "1 of 3 topics achieved" : "1/3 chủ đề đạt yêu cầu",
        ),
      ).toBeVisible();
      expect(
        screen.getByRole("link", {
          name: locale === "en" ? "Practice generally" : "Luyện tổng quát",
        }),
      ).toHaveAttribute(
        "href",
        "/interviews/new/interviewer?roleProfileId=22222222-2222-4222-8222-222222222222",
      );
      fireEvent.click(screen.getByRole("button", { name: /System design/ }));
      expect(
        screen.getByRole("link", {
          name: locale === "en" ? "Practice this topic" : "Luyện chủ đề này",
        }),
      ).toHaveAttribute(
        "href",
        "/interviews/new/interviewer?roleProfileId=22222222-2222-4222-8222-222222222222&focusItemId=55555555-5555-4555-8555-555555555555",
      );
      expect(
        screen.getAllByText(
          locale === "en" ? "Not evaluated yet" : "Chưa được đánh giá",
        ).length,
      ).toBeGreaterThan(0);
      expect(
        screen.queryByRole("button", {
          name: locale === "en" ? "Regenerate roadmap" : "Tạo lại lộ trình",
        }),
      ).not.toBeInTheDocument();
    },
  );
  it.each(["pending", "generating", "failed"] as const)(
    "prioritizes %s over old published topics and null progress",
    async (generation_status) => {
      mocks.get.mockImplementation((path: string) =>
        Promise.resolve(
          ok(
            path.endsWith("/roadmap")
              ? roadmapFixture({
                  generation_status,
                  progress_percent: null,
                  practice_started: false,
                })
              : profileFixture,
          ),
        ),
      );
      renderScreen();
      await screen.findByText(
        generation_status === "generating"
          ? "Preparing your roadmap"
          : generation_status === "failed"
            ? "Your roadmap is not ready"
            : "Your roadmap starts with your requirements",
      );
      expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: /View topic:/ }),
      ).not.toBeInTheDocument();
    },
  );
  it("does not present an empty ready plan as completed", async () => {
    mocks.get.mockImplementation((path: string) =>
      Promise.resolve(
        ok(
          path.endsWith("/roadmap")
            ? roadmapFixture({
                items: [],
                total_items: 0,
                completed_items: 0,
                progress_percent: null,
              })
            : profileFixture,
        ),
      ),
    );
    renderScreen();
    await screen.findByText("No topics in this roadmap");
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
  });
  it("shows a legacy recovery after a refused retry", async () => {
    mocks.get.mockImplementation((path: string) =>
      Promise.resolve(
        ok(
          path.endsWith("/roadmap")
            ? roadmapFixture({
                generation_status: "pending",
                practice_started: false,
              })
            : profileFixture,
        ),
      ),
    );
    mocks.post.mockRejectedValue(
      new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
        status: 409,
        statusText: "Conflict",
        config: { headers: new AxiosHeaders() },
        headers: {},
        data: { error: { code: "ROADMAP_NOT_RETRYABLE" } },
      }),
    );
    renderScreen();
    fireEvent.click(
      await screen.findByRole("button", { name: "Prepare roadmap" }),
    );
    await screen.findByText("This profile has no learning roadmap");
    expect(
      screen.getByRole("link", { name: "Create Role Profile" }),
    ).toBeVisible();
  });
  it("warns before regeneration and waits for the real operation", async () => {
    mocks.get.mockImplementation((path: string) =>
      Promise.resolve(
        ok(
          path.endsWith("/roadmap")
            ? roadmapFixture({ practice_started: false })
            : profileFixture,
        ),
      ),
    );
    mocks.post.mockResolvedValue(
      ok({
        roadmap_id: roadmapFixture().id,
        role_profile_id: profileId,
        generation_status: "ready",
      }),
    );
    renderScreen();
    fireEvent.click(
      await screen.findByRole("button", { name: "Regenerate roadmap" }),
    );
    expect(mocks.post).not.toHaveBeenCalled();
    fireEvent.click(
      within(screen.getByRole("dialog")).getByRole("button", {
        name: "Regenerate roadmap",
      }),
    );
    await waitFor(() =>
      expect(mocks.post).toHaveBeenCalledWith(
        `/role-profiles/${profileId}/roadmap/regenerate`,
        undefined,
        { timeout: 210_000 },
      ),
    );
  });
  it.each(["updating", "delayed"] as const)(
    "announces %s without inventing a new result",
    async (update_state) => {
      mocks.get.mockImplementation((path: string) =>
        Promise.resolve(
          ok(
            path.endsWith("/roadmap")
              ? roadmapFixture({ update_state, pending_result_count: 2 })
              : profileFixture,
          ),
        ),
      );
      renderScreen();
      expect(
        await screen.findByText(
          update_state === "updating"
            ? /latest results are being added/
            : /latest session has not been added/,
        ),
      ).toBeVisible();
    },
  );
  it("keeps a high provisional score distinct from achievement", () => {
    renderWithIntl(
      <TopicScore
        item={{ ...roadmapFixture().items[1], score: 95 }}
        windowSize={5}
        threshold={80}
      />,
    );
    expect(screen.getByText(/Provisional score/)).toBeVisible();
  });
  it("explains rounded 80 when the authoritative status is below threshold", () => {
    renderWithIntl(
      <TopicScore
        item={{
          ...roadmapFixture().items[1],
          score: 80,
          evidence_question_count: 5,
        }}
        windowSize={5}
        threshold={80}
      />,
    );
    expect(screen.getByText(/Just below 80/)).toBeVisible();
  });
  it("does not second-guess an exact achieved score of 80", () => {
    renderWithIntl(
      <TopicScore
        item={{ ...roadmapFixture().items[0], score: 80 }}
        windowSize={5}
        threshold={80}
      />,
    );
    expect(screen.queryByText(/Just below/)).not.toBeInTheDocument();
  });
});
it("uses bounded generation and catch-up polling", () => {
  const generating = roadmapFixture({ generation_status: "generating" });
  expect(roadmapPollInterval(generating, 0)).toBe(3000);
  expect(roadmapPollInterval(generating, 30_000)).toBe(5000);
  expect(roadmapPollInterval(generating, 300_000)).toBe(false);
  expect(
    roadmapPollInterval(roadmapFixture({ update_state: "updating" }), 0),
  ).toBe(10_000);
  expect(
    roadmapPollInterval(roadmapFixture({ update_state: "updating" }), 120_000),
  ).toBe(false);
  expect(
    roadmapPollInterval(roadmapFixture({ update_state: "delayed" }), 0),
  ).toBe(false);
});
it("validates null scores and never trusts hidden response fields", () => {
  const parsed = roadmapSchema.parse({
    ...roadmapFixture(),
    expected_answer: "private",
  });
  expect(parsed).not.toHaveProperty("expected_answer");
  expect(parsed.items[2].score).toBeNull();
});
it("walks history pagination to identify the actual session, including old evidence", async () => {
  const row = {
    interview_id: sessionId,
    question_number: 1,
    result: "scored",
    score: 70,
    question_text: "SQL?",
    evaluated_at: "2026-11-13T05:00:00Z",
    in_current_window: false,
  };
  mocks.get
    .mockResolvedValueOnce(
      ok({
        items: [{ ...row, interview_id: profileId }],
        pagination: { total: 2, offset: 0, limit: 100 },
      }),
    )
    .mockResolvedValueOnce(
      ok({ items: [row], pagination: { total: 2, offset: 1, limit: 100 } }),
    );
  expect(await getSessionTopicEvidence(profileId, itemId, sessionId)).toEqual([
    row,
  ]);
  expect(mocks.get).toHaveBeenCalledTimes(2);
});
