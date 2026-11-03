import { expect, test } from "@playwright/test";
import { cors } from "./fixtures/avatar-api";
import { mockLiveServer, silentWav } from "./fixtures/live-interview";
import { mockRoomPreview, roomId } from "./fixtures/room-preview-api";

const room = `/interviews/${roomId}/room`;
const questions = [
  { text: "Tell me about a system you designed.", audio: silentWav() },
  { text: "How did you measure its latency?" },
  { text: "What would you change next time?" },
];

test.use({
  launchOptions: {
    args: [
      "--use-fake-device-for-media-stream",
      "--use-fake-ui-for-media-stream",
    ],
  },
});

function report(status: "pending" | "completed") {
  return {
    interview_id: roomId,
    type: "practice",
    status,
    overall_score: status === "completed" ? 74 : null,
    started_at: "2026-11-02T08:00:00Z",
    completed_at: "2026-11-02T08:20:00Z",
    summary: status === "completed" ? "Clear and structured." : null,
    breakdown: null,
    questions: null,
  };
}

test("a practice interview runs over the live socket, survives a reconnect and opens the polled report", async ({
  page,
}) => {
  const calls = await mockRoomPreview(page, { live: false });
  const server = await mockLiveServer(page, {
    sessionId: roomId,
    questions,
    dropWhenListeningAt: 3,
  });
  let reportReads = 0;
  await page.route(
    `http://localhost:8080/interviews/${roomId}/report`,
    (route) => {
      reportReads++;
      return route.fulfill({
        headers: cors,
        json: {
          success: true,
          data: report(reportReads < 2 ? "pending" : "completed"),
        },
      });
    },
  );
  // Mirror the autoplay policy deterministically: play() needs a real tap first. Direct navigation to
  // the room has none, so the room must offer its one-tap Play.
  await page.addInitScript(() => {
    const play = HTMLMediaElement.prototype.play;
    let gesture = false;
    addEventListener(
      "pointerdown",
      (event) => {
        if (event.isTrusted) gesture = true;
      },
      true,
    );
    HTMLMediaElement.prototype.play = function () {
      return gesture
        ? play.call(this)
        : Promise.reject(
            new DOMException("Autoplay blocked", "NotAllowedError"),
          );
    };
  });
  await page.goto(room);
  const stage = page.getByRole("region", {
    name: "Interview room",
    exact: true,
  });
  await expect(page.getByText("Question 1 of 3")).toBeVisible();
  await expect(
    page.getByLabel("Live transcript").getByText(questions[0].text),
  ).toBeVisible();

  // Q1: audio turn, autoplay refused, one tap plays it; then a typed answer.
  await page.getByRole("button", { name: "Play question" }).click();
  await expect(page.getByText("Your turn to answer")).toBeVisible();
  await expect(stage).toHaveAttribute("data-performance-state", "listening");
  await page.getByRole("button", { name: "Type instead" }).click();
  await page.getByLabel("Your answer").fill("I designed a rate limiter.");
  await page.getByRole("button", { name: "Send answer" }).click();
  await expect(page.getByText("I designed a rate limiter.")).toBeVisible();

  // Q2: text-only turn, recorded answer from the fake microphone.
  await expect(page.getByText("Question 2 of 3")).toBeVisible();
  await page.getByRole("button", { name: "Answer by voice" }).first().click();
  await expect(
    page.getByRole("timer", { name: "Time left to answer" }),
  ).toContainText(/[23]:\d\d left/);
  await page.waitForTimeout(1200);
  await page.getByRole("button", { name: "Done answering" }).click();
  await expect(
    page.getByText("Spoken answer from the e2e server."),
  ).toBeVisible();
  const chunks = server.frames[0].filter(
    (frame) => frame.type === "answer.audio",
  );
  expect(chunks.length).toBeGreaterThan(0);
  expect(chunks[0].data).toMatchObject({
    position: 2,
    chunk: 0,
    mime_type: "audio/webm;codecs=opus",
  });
  expect(chunks.at(-1)?.data.final).toBe(true);
  expect(new Set(chunks.map((frame) => frame.id)).size).toBe(1);

  // Q3: the server drops the socket while listening. The room reconnects, does not replay the turn,
  // and continues from the server's state.
  await expect(page.getByText("Question 3 of 3")).toBeVisible();
  await expect(page.getByText("Connection lost. Reconnecting…")).toBeVisible();
  await expect(page.getByText("Your turn to answer")).toBeVisible();
  expect(server.connections).toBe(2);
  // The turn is reported again with a fresh id: the server remembers the first one.
  const playedBefore = server.frames[0].filter(
    (frame) => frame.type === "turn.played" && frame.data.position === 3,
  );
  const playedAfter = server.frames[1].filter(
    (frame) => frame.type === "turn.played",
  );
  expect(playedAfter).toHaveLength(1);
  expect(playedAfter[0].id).not.toBe(playedBefore[0].id);
  await page.getByRole("button", { name: "Type instead" }).click();
  await page.getByLabel("Your answer").fill("Shard the counters.");
  await page.getByRole("button", { name: "Send answer" }).click();

  await expect(page.getByText("Interview complete")).toBeVisible();
  await expect(page).toHaveURL(`/reports/${roomId}`);
  await expect(
    page.getByText("We'll update this page automatically."),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Overall score" }),
  ).toBeVisible({ timeout: 10000 });
  expect(server.completed).toBe(true);
  expect(calls.writes).toEqual([]);
});

test("a recruitment room shows questions but never answer transcripts", async ({
  page,
}) => {
  await mockRoomPreview(page, { recruitment: true, live: false });
  await mockLiveServer(page, {
    sessionId: roomId,
    sessionType: "recruitment",
    questions: [{ text: "Why this team?" }, { text: "Any questions?" }],
  });
  await page.goto(room);
  await expect(page.getByText("Recruitment room")).toBeVisible();
  await page.getByRole("button", { name: "Type instead" }).click();
  await page.getByLabel("Your answer").fill("Because of the product.");
  await page.getByRole("button", { name: "Send answer" }).click();
  await expect(page.getByText("Question 2 of 2")).toBeVisible();
  await expect(page.getByText("Because of the product.")).toHaveCount(0);
  await expect(page.getByText("The interviewer's questions")).toBeVisible();
});

test("a follow-up keeps the core question's progress and is labelled", async ({
  page,
}) => {
  await mockRoomPreview(page, { live: false });
  const server = await mockLiveServer(page, {
    sessionId: roomId,
    questions: [
      {
        text: "How would you design a rate limiter?",
        followUp: "Which algorithm would you pick for bursty traffic?",
      },
      { text: "How do you test it?" },
    ],
  });
  await page.goto(room);
  const followUpLabel = page.getByTestId("follow-up-label");
  await expect(page.getByText("Question 1 of 2")).toBeVisible();
  await expect(followUpLabel).toHaveCount(0);
  await page.getByRole("button", { name: "Type instead" }).click();
  await page.getByLabel("Your answer").fill("A token bucket per client.");
  await page.getByRole("button", { name: "Send answer" }).click();

  // The follow-up is turn 2 of the conversation but still question 1 of 2.
  await expect(followUpLabel).toHaveText("Follow-up");
  await expect(page.getByText("Question 1 of 2")).toBeVisible();
  await expect(
    page
      .getByLabel("Live transcript")
      .getByText("Which algorithm would you pick for bursty traffic?"),
  ).toBeVisible();
  await page.getByLabel("Your answer").fill("Token bucket, it allows bursts.");
  await page.getByRole("button", { name: "Send answer" }).click();

  // The typed composer stays open between turns.
  await expect(page.getByText("Question 2 of 2")).toBeVisible();
  await expect(followUpLabel).toHaveCount(0);
  await page.getByLabel("Your answer").fill("Table-driven tests.");
  await page.getByRole("button", { name: "Send answer" }).click();
  await expect(page.getByText("Interview complete")).toBeVisible();
  const answers = server.frames[0].filter(
    (frame) => frame.type === "answer.text",
  );
  expect(answers.map((frame) => frame.data.position)).toEqual([1, 2, 3]);
});

test("a follow-up lost in a reconnect is dropped and the next core question takes its place", async ({
  page,
}) => {
  await mockRoomPreview(page, { live: false });
  const server = await mockLiveServer(page, {
    sessionId: roomId,
    questions: [
      {
        text: "How would you design a rate limiter?",
        followUp: "Which algorithm would you pick for bursty traffic?",
      },
      { text: "How do you test it?" },
    ],
    dropWhenListeningAt: 2,
  });
  await page.goto(room);
  await page.getByRole("button", { name: "Type instead" }).click();
  await page.getByLabel("Your answer").fill("A token bucket per client.");
  await page.getByRole("button", { name: "Send answer" }).click();
  await expect(page.getByTestId("follow-up-label")).toHaveText("Follow-up");

  // The socket drops while the Candidate is asked the follow-up.
  await expect(page.getByText("Question 2 of 2")).toBeVisible();
  await expect(page.getByTestId("follow-up-label")).toHaveCount(0);
  await expect(
    page.getByLabel("Live transcript").getByText("How do you test it?"),
  ).toBeVisible();
  await expect(page.getByText("Your turn to answer")).toBeVisible();
  expect(server.connections).toBe(2);
  const typeInstead = page.getByRole("button", { name: "Type instead" });
  if (await typeInstead.isVisible()) await typeInstead.click();
  await page.getByLabel("Your answer").fill("Table-driven tests.");
  await page.getByRole("button", { name: "Send answer" }).click();
  await expect(page.getByText("Interview complete")).toBeVisible();
  const answers = server.frames
    .flat()
    .filter((frame) => frame.type === "answer.text");
  expect(answers.map((frame) => frame.data)).toEqual([
    { position: 1, text: "A token bucket per client." },
    { position: 2, text: "Table-driven tests." },
  ]);
});
