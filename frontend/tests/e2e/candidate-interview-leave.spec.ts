import { expect, test } from "@playwright/test";
import { cors } from "./fixtures/avatar-api";
import { mockLiveServer } from "./fixtures/live-interview";
import { mockRoomPreview, roomId } from "./fixtures/room-preview-api";

// Leaving ends the interview (KAN-215, decision 2026-11-03): no pause. The room asks first, then submits
// with POST /interviews/{id}/complete, shows "submitting…" and opens the report, which says the
// Candidate left and counts the unanswered questions.

const room = `/interviews/${roomId}/room`;
const questions = [
  { text: "Tell me about a system you designed." },
  { text: "How did you measure its latency?" },
  { text: "What would you change next time?" },
];

function leftReport() {
  return {
    interview_id: roomId,
    type: "practice",
    status: "completed",
    overall_score: 61,
    started_at: "2026-11-03T08:00:00Z",
    completed_at: "2026-11-03T08:06:00Z",
    end_reason: "left",
    question_count: 3,
    summary: "A clear first answer.",
    breakdown: [],
    questions: [
      {
        position: 1,
        question_number: 1,
        kind: "core",
        question: questions[0].text,
        answer: "I designed a rate limiter.",
        feedback: "Name the trade-offs.",
      },
    ],
  };
}

test("Leave asks first, then submits at once and opens the report of a left interview", async ({
  page,
}) => {
  const calls = await mockRoomPreview(page, { live: false });
  const server = await mockLiveServer(page, { sessionId: roomId, questions });
  let release: () => void = () => {};
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  const ends: string[] = [];
  // Registered after the preview mock, so it answers first: the leave is held until the test releases
  // it, to see the room while it submits.
  await page.route(
    `http://localhost:8080/interviews/${roomId}/complete`,
    async (route) => {
      if (route.request().method() === "OPTIONS")
        return route.fulfill({ status: 204, headers: cors });
      ends.push(route.request().method());
      await held;
      return route.fulfill({
        headers: cors,
        json: {
          success: true,
          data: {
            id: roomId,
            completed_at: "2026-11-03T08:06:00Z",
            end_reason: "left",
            evaluation_status: "pending",
          },
        },
      });
    },
  );
  await page.route(
    `http://localhost:8080/interviews/${roomId}/report`,
    (route) =>
      route.fulfill({
        headers: cors,
        json: { success: true, data: leftReport() },
      }),
  );

  await page.goto(room);
  await expect(page.getByText("Question 1 of 3")).toBeVisible();
  await page.getByRole("button", { name: "Type instead" }).click();
  await page.getByLabel("Your answer").fill("I designed a rate limiter.");
  await page.getByRole("button", { name: "Send answer" }).click();
  await expect(page.getByText("Question 2 of 3")).toBeVisible();

  // The confirmation explains what leaving does; keeping on changes nothing.
  await page.getByRole("button", { name: "Leave", exact: true }).click();
  const dialog = page.getByRole("dialog", {
    name: "Leave and end this interview?",
  });
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText("It is not a pause");
  await expect(dialog).toContainText(
    "Questions you haven't answered stay unanswered.",
  );
  await expect(dialog).toContainText(
    "The coins for this practice are not refunded.",
  );
  await dialog.getByRole("button", { name: "Keep interviewing" }).click();
  await expect(dialog).toBeHidden();
  expect(ends).toEqual([]);
  await expect(page.getByText("Question 2 of 3")).toBeVisible();

  // Confirming submits at once and shows the room submitting until the server answers.
  await page.getByRole("button", { name: "Leave", exact: true }).click();
  await dialog.getByRole("button", { name: "Leave and submit" }).click();
  await expect(page.getByText("Submitting your interview…")).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Submitting…" }),
  ).toBeDisabled();
  expect(ends).toEqual(["POST"]);
  release();

  await expect(page.getByText("Interview complete")).toBeVisible();
  await expect(page).toHaveURL(`/reports/${roomId}`);
  await expect(
    page.getByRole("heading", { name: "You left the interview early" }),
  ).toBeVisible();
  await expect(page.getByTestId("report-end-notice")).toContainText(
    "1 of 3 questions answered; 2 questions not answered.",
  );
  expect(ends).toEqual(["POST"]);
  expect(server.completed).toBe(false);
  expect(calls.writes).toEqual([]);
});

test("a failed leave keeps the interview running and can be tried again", async ({
  page,
}) => {
  await mockRoomPreview(page, { live: false });
  await mockLiveServer(page, { sessionId: roomId, questions });
  let attempts = 0;
  await page.route(
    `http://localhost:8080/interviews/${roomId}/complete`,
    (route) => {
      if (route.request().method() === "OPTIONS")
        return route.fulfill({ status: 204, headers: cors });
      attempts++;
      if (attempts === 1) return route.abort("internetdisconnected");
      return route.fulfill({
        headers: cors,
        json: {
          success: true,
          data: {
            id: roomId,
            completed_at: "2026-11-03T08:06:00Z",
            end_reason: "left",
            evaluation_status: "pending",
          },
        },
      });
    },
  );
  await page.goto(room);
  await expect(page.getByText("Question 1 of 3")).toBeVisible();
  await page.getByRole("button", { name: "Leave", exact: true }).click();
  await page.getByRole("button", { name: "Leave and submit" }).click();
  await expect(page.getByText("The interview was not ended")).toBeVisible();
  await expect(page.getByText("Question 1 of 3")).toBeVisible();

  await page.getByRole("button", { name: "Leave", exact: true }).click();
  await page.getByRole("button", { name: "Leave and submit" }).click();
  await expect(page).toHaveURL(`/reports/${roomId}`);
  expect(attempts).toBe(2);
});
