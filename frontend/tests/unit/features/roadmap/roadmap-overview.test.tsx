import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderWithIntl } from "@tests/helpers/intl";
import {
  roadmapFixture,
  profileId,
  secondItemId,
  itemId,
} from "@tests/fixtures/roadmap";
import { RoadmapOverview } from "@/features/roadmap/components/roadmap-overview";
import { nextPracticeTopic } from "@/features/roadmap/lib/next-practice";

const mocks = vi.hoisted(() => ({ get: vi.fn() }));
vi.mock("@/lib/api/client", () => ({ apiClient: mocks }));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => ({ id: "user" }),
}));
const otherId = "88888888-8888-4888-8888-888888888888";
const summary = (
  id = profileId,
  title = "Backend Engineer",
  status = "confirmed",
) => ({
  id,
  title,
  status,
  seniorityLevel: "mid",
  createdAt: "2026-11-13T00:00:00Z",
});
const ok = (data: unknown) => ({ data: { success: true, data } });
const page = (items = [summary()], total = items.length) =>
  ok({ items, pagination: { limit: 20, offset: 0, total } });
function mount(locale: "en" | "vi" = "en") {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  renderWithIntl(
    <QueryClientProvider client={client}>
      <RoadmapOverview />
    </QueryClientProvider>,
    { locale },
  );
  return client;
}
beforeEach(() => {
  mocks.get.mockReset();
  mocks.get.mockImplementation((path: string) =>
    Promise.resolve(path === "/role-profiles" ? page() : ok(roadmapFixture())),
  );
});
afterEach(() => vi.useRealTimers());

function pointer(
  target: Element,
  type: "pointerover" | "pointerout",
  pointerType = "mouse",
) {
  const event = new MouseEvent(type, { bubbles: true });
  Object.defineProperty(event, "pointerType", { value: pointerType });
  fireEvent(target, event);
}

describe("Roadmap overview", () => {
  it("previews topics, selects their real description and preserves practice context without fetching again", async () => {
    mount();
    const toggle = await screen.findByRole("button", {
      name: "Preview topics for Backend Engineer",
    });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(
      screen.queryByRole("button", {
        name: "View topic: SQL & data modelling",
      }),
    ).toBeNull();
    fireEvent.click(toggle);
    const article = toggle.closest("article");
    if (!article) throw new Error("Missing roadmap row");
    const row = within(article);
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    const achieved = row.getByRole("button", {
      name: "View topic: SQL & data modelling",
    });
    expect(achieved).toHaveTextContent("Achieved");
    expect(
      row.getByRole("button", { name: "View topic: Go concurrency" }),
    ).not.toHaveTextContent("Achieved");
    fireEvent.click(achieved);
    expect(
      row.getByText("Indexes, joins, transactions and query performance."),
    ).toBeInTheDocument();
    expect(
      row.getByRole("link", { name: "Practice this topic" }),
    ).toHaveAttribute(
      "href",
      `/interviews/new/interviewer?roleProfileId=${profileId}&focusItemId=${itemId}`,
    );
    expect(mocks.get).toHaveBeenCalledTimes(2);
    achieved.focus();
    fireEvent.keyDown(achieved, { key: "Escape" });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(toggle).toHaveFocus();
  });

  it("opens after a deliberate hover and closes on leave", async () => {
    mount();
    const toggle = await screen.findByRole("button", {
      name: "Preview topics for Backend Engineer",
    });
    const article = toggle.closest("article");
    if (!article) throw new Error("Missing roadmap row");
    vi.useFakeTimers();
    pointer(article, "pointerover");
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    act(() => vi.advanceTimersByTime(120));
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    pointer(article, "pointerout");
    act(() => vi.advanceTimersByTime(180));
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    pointer(article, "pointerover");
    pointer(article, "pointerout");
    act(() => vi.advanceTimersByTime(300));
    expect(toggle).toHaveAttribute("aria-expanded", "false");
  });

  it("supports a first touch tap without accidental hover activation", async () => {
    mount();
    const toggle = await screen.findByRole("button", {
      name: "Preview topics for Backend Engineer",
    });
    const article = toggle.closest("article");
    if (!article) throw new Error("Missing roadmap row");
    vi.useFakeTimers();
    pointer(article, "pointerover", "touch");
    act(() => vi.advanceTimersByTime(300));
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    fireEvent.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    pointer(article, "pointerout", "touch");
    act(() => vi.advanceTimersByTime(300));
    expect(toggle).toHaveAttribute("aria-expanded", "true");
  });

  it("keeps one preview open and protects a focused topic from unrelated hover", async () => {
    mocks.get.mockImplementation((path: string) =>
      Promise.resolve(
        path === "/role-profiles"
          ? page([summary(), summary(otherId, "Frontend Engineer")])
          : ok(
              roadmapFixture({
                role_profile_id: path.includes(otherId) ? otherId : profileId,
              }),
            ),
      ),
    );
    mount();
    const first = await screen.findByRole("button", {
      name: "Preview topics for Backend Engineer",
    });
    const second = await screen.findByRole("button", {
      name: "Preview topics for Frontend Engineer",
    });
    fireEvent.click(first);
    const firstArticle = first.closest("article");
    const secondArticle = second.closest("article");
    if (!firstArticle || !secondArticle)
      throw new Error("Missing roadmap rows");
    within(firstArticle)
      .getByRole("button", { name: "View topic: Go concurrency" })
      .focus();
    vi.useFakeTimers();
    pointer(secondArticle, "pointerover");
    act(() => vi.advanceTimersByTime(300));
    expect(first).toHaveAttribute("aria-expanded", "true");
    expect(second).toHaveAttribute("aria-expanded", "false");
    fireEvent.click(second);
    expect(first).toHaveAttribute("aria-expanded", "false");
    expect(second).toHaveAttribute("aria-expanded", "true");
  });

  it("shows achieved topics independently when practice did not follow the rail order", async () => {
    const data = roadmapFixture();
    data.items[2].learning_status = "achieved";
    mocks.get.mockImplementation((path: string) =>
      Promise.resolve(path === "/role-profiles" ? page() : ok(data)),
    );
    mount();
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Preview topics for Backend Engineer",
      }),
    );
    expect(
      screen.getByRole("button", { name: "View topic: SQL & data modelling" }),
    ).toHaveTextContent("Achieved");
    expect(
      screen.getByRole("button", { name: "View topic: System design" }),
    ).toHaveTextContent("Achieved");
    expect(
      screen.getByRole("button", { name: "View topic: Go concurrency" }),
    ).not.toHaveTextContent("Achieved");
  });
  it.each(["en", "vi"] as const)(
    "shows authoritative progress and practice links retaining their role and focus in %s",
    async (locale) => {
      mount(locale);
      expect(await screen.findByRole("progressbar")).toHaveAttribute(
        "aria-valuenow",
        "33.33",
      );
      expect(
        screen.getByRole("heading", { name: "Go concurrency" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("link", {
          name: locale === "en" ? "Practice this topic" : "Luyện chủ đề này",
        }),
      ).toHaveAttribute(
        "href",
        `/interviews/new/interviewer?roleProfileId=${profileId}&focusItemId=${secondItemId}`,
      );
      expect(
        screen.getByRole("link", {
          name: locale === "en" ? "Practice generally" : "Luyện tổng quát",
        }),
      ).toHaveAttribute(
        "href",
        `/interviews/new/interviewer?roleProfileId=${profileId}`,
      );
      expect(
        screen.getByText(locale === "en" ? "33%" : "33%", { exact: true }),
      ).toBeInTheDocument();
    },
  );
  it("shows a create action for an empty library without requesting roadmap data", async () => {
    mocks.get.mockResolvedValue(page([]));
    mount();
    expect(
      await screen.findByText("Your next role starts here"),
    ).toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("progressbar")).toBeNull();
  });
  it("recovers from a failed library read", async () => {
    mocks.get.mockRejectedValueOnce(new Error("offline"));
    mount();
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Your roadmaps could not be loaded",
    );
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByRole("progressbar")).toBeVisible();
  });
  it("isolates an individual roadmap error and retries it without blocking healthy profiles", async () => {
    let failing = true;
    mocks.get.mockImplementation((path: string) =>
      path === "/role-profiles"
        ? Promise.resolve(
            page([summary(), summary(otherId, "Frontend Engineer")]),
          )
        : path.includes(profileId) && failing
          ? Promise.reject(new Error("offline"))
          : Promise.resolve(ok(roadmapFixture({ role_profile_id: otherId }))),
    );
    mount();
    const library = await screen.findByRole("list", { name: "All roadmaps" });
    expect(await within(library).findByRole("alert")).toBeInTheDocument();
    expect(
      await within(library).findByRole("progressbar", {
        name: "Progress for Frontend Engineer",
      }),
    ).toBeVisible();
    failing = false;
    fireEvent.click(within(library).getByRole("button", { name: "Try again" }));
    await waitFor(() =>
      expect(within(library).getAllByRole("progressbar")).toHaveLength(2),
    );
  });
  it.each(["pending", "generating", "failed"] as const)(
    "does not invent progress or practice actions while %s",
    async (generation_status) => {
      mocks.get.mockImplementation((path: string) =>
        Promise.resolve(
          path === "/role-profiles"
            ? page()
            : ok(roadmapFixture({ generation_status, progress_percent: null })),
        ),
      );
      mount();
      const library = await screen.findByRole("list", { name: "All roadmaps" });
      expect(
        await within(library).findByRole("link", { name: "View status" }),
      ).toHaveAttribute("href", `/role-profiles/${profileId}/roadmap`);
      expect(screen.queryByRole("progressbar")).toBeNull();
      expect(
        screen.queryByRole("link", { name: "Practice this topic" }),
      ).toBeNull();
    },
  );
  it("lets an unconfirmed profile return to review without requesting its roadmap", async () => {
    mocks.get.mockResolvedValue(
      page([summary(profileId, "Backend Engineer", "review")]),
    );
    mount();
    expect(
      await screen.findByRole("link", { name: "Review profile" }),
    ).toHaveAttribute("href", `/role-profiles/${profileId}`);
    expect(mocks.get).toHaveBeenCalledTimes(1);
  });
  it("keeps the previous progress visible when background refresh fails", async () => {
    const client = mount();
    await screen.findByRole("progressbar");
    mocks.get.mockRejectedValue(new Error("offline"));
    await client.invalidateQueries({
      queryKey: ["roadmap", "user", profileId],
    });
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "The last available progress is shown",
    );
    expect(screen.getByRole("progressbar")).toHaveAttribute(
      "aria-valuenow",
      "33.33",
    );
  });
  it("loads another page with bounded profile requests", async () => {
    mocks.get.mockImplementation(
      (path: string, options?: { params?: { offset: number } }) =>
        Promise.resolve(
          path === "/role-profiles"
            ? page(
                options?.params?.offset === 20
                  ? [summary(otherId, "Next role")]
                  : [summary()],
                21,
              )
            : ok(roadmapFixture()),
        ),
    );
    mount();
    await screen.findByRole("progressbar");
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    expect(
      await screen.findByRole("heading", { name: "Next role" }),
    ).toBeInTheDocument();
    expect(mocks.get).toHaveBeenCalledWith(
      "/role-profiles",
      expect.objectContaining({ params: { limit: 20, offset: 20 } }),
    );
  });
  it("offers general practice when all topics are achieved", async () => {
    const data = roadmapFixture();
    mocks.get.mockImplementation((path: string) =>
      Promise.resolve(
        path === "/role-profiles"
          ? page()
          : ok(
              roadmapFixture({
                learning_status: "completed",
                progress_percent: 100,
                completed_items: 3,
                items: data.items.map((item) => ({
                  ...item,
                  learning_status: "achieved",
                })),
              }),
            ),
      ),
    );
    mount();
    expect(
      await screen.findByText("Keep your skills fresh"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Practice this topic" }),
    ).toBeNull();
    expect(
      screen.getByRole("link", { name: "Practice generally" }),
    ).toHaveAttribute(
      "href",
      `/interviews/new/interviewer?roleProfileId=${profileId}`,
    );
  });

  it("provides a way back when the final page disappears after a profile is removed", async () => {
    mocks.get.mockImplementation(
      (path: string, options?: { params?: { offset: number } }) =>
        Promise.resolve(
          path === "/role-profiles"
            ? options?.params?.offset === 20
              ? page([], 20)
              : page([summary()], 21)
            : ok(roadmapFixture()),
        ),
    );
    mount();
    await screen.findByRole("progressbar");
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    expect(
      await screen.findByText("These profiles are no longer on this page"),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Back to first page" }));
    expect(await screen.findByRole("progressbar")).toBeVisible();
  });

  it("keeps unavailable progress distinct from zero", async () => {
    mocks.get.mockImplementation((path: string) =>
      Promise.resolve(
        path === "/role-profiles"
          ? page()
          : ok(
              roadmapFixture({
                items: [],
                total_items: 0,
                completed_items: 0,
                progress_percent: null,
              }),
            ),
      ),
    );
    mount();
    expect(await screen.findByText("Not available yet")).toBeInTheDocument();
    expect(screen.queryByRole("progressbar")).toBeNull();
    expect(screen.queryByText("0%", { exact: true })).toBeNull();
    expect(
      screen.queryByRole("link", { name: "Practice generally" }),
    ).toBeNull();
  });
});

describe("Practice suggestion", () => {
  it("does not treat a high provisional score as achieved", () => {
    const roadmap = roadmapFixture();
    roadmap.items[1].score = 99;
    expect(nextPracticeTopic(roadmap)?.id).toBe(secondItemId);
    expect(
      nextPracticeTopic(roadmapFixture({ generation_status: "generating" })),
    ).toBeUndefined();
  });
});
