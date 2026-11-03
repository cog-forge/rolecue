import { expect, type Page } from "@playwright/test";
import { avatarItem, cors } from "./avatar-api";
import { mockLiveServer } from "./live-interview";
export const roomId = "aaaaaaaa-aaaa-4aaa-8aaa-000000000053";
export async function mockRoomPreview(
  page: Page,
  options: {
    recruitment?: boolean;
    avatar?: boolean;
    error?: boolean;
    completed?: boolean;
    /** Serve a one-question live socket so the room never reaches a real API (default). */
    live?: boolean;
    companyInterviewerError?: boolean;
  } = {},
) {
  const calls = {
    reads: 0,
    details: 0,
    companyInterviewerReads: 0,
    inventoryReads: [] as string[],
    writes: [] as string[],
    /** POST /interviews/{id}/complete: the room's Leave (docs/interview-ws-protocol.md §3.1). */
    ends: 0,
  };
  if (options.live !== false)
    await mockLiveServer(page, {
      sessionId: roomId,
      sessionType: options.recruitment ? "recruitment" : "practice",
      questions: [{ text: "Walk me through a recent project." }],
    });
  await page.route("http://localhost:8080/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (request.method() === "OPTIONS")
      return route.fulfill({ status: 204, headers: cors });
    const ok = (data: unknown) =>
      route.fulfill({ headers: cors, json: { success: true, data } });
    if (
      request.method() === "POST" &&
      path === `/interviews/${roomId}/complete`
    ) {
      calls.ends++;
      return ok({
        id: roomId,
        completed_at: "2026-11-03T09:00:00Z",
        end_reason: "left",
        evaluation_status: "pending",
      });
    }
    if (request.method() !== "GET") {
      calls.writes.push(`${request.method()} ${path}`);
      return route.fulfill({
        status: 405,
        headers: cors,
        json: { success: false, error: { code: "METHOD_NOT_ALLOWED" } },
      });
    }
    if (path === "/auth/me")
      return ok({
        id: "11111111-1111-4111-8111-111111111111",
        email: "candidate@example.com",
        full_name: "Preview Candidate",
        image: null,
        role: "candidate",
        email_verified: true,
        is_locked: false,
        onboarding_role_selected: true,
        onboarding_completed: true,
      });
    if (path === `/interviews/${roomId}`) {
      calls.reads++;
      if (options.error)
        return route.fulfill({
          status: 404,
          headers: cors,
          json: {
            success: false,
            error: {
              code: "INTERVIEW_NOT_FOUND",
              message: "Internal server information",
            },
          },
        });
      return ok({
        id: roomId,
        type: options.recruitment ? "recruitment" : "practice",
        avatar_id: options.avatar ? avatarItem(1).id : null,
        completed_at: options.completed ? "2026-10-29T09:00:00Z" : null,
        current_question: { position: 1, content: "Old REST question" },
        turns: [
          { position: 1, question: "Old question", answer: "Old answer" },
        ],
        question_count: 8,
        answered_count: 1,
      });
    }
    if (path === `/interviews/${roomId}/interviewer`) {
      calls.companyInterviewerReads++;
      if (options.companyInterviewerError)
        return route.fulfill({
          status: 503,
          headers: cors,
          json: {
            success: false,
            error: {
              code: "AVATAR_ASSET_UNAVAILABLE",
              message: "Internal server information",
            },
          },
        });
      return ok({
        name: "Acme interviewer",
        voice_name: "Company voice",
        model_access: {
          url: "https://s3.test/company-interviewer.vrm",
          method: "GET",
          headers: {},
          expires_at: "2099-01-01T00:00:00Z",
        },
      });
    }
    if (path.startsWith("/inventory")) calls.inventoryReads.push(path);
    if (path.startsWith("/inventory/avatars/")) {
      calls.details++;
      return ok({
        ...avatarItem(1),
        model_access: {
          url: `https://s3.test/avatar-${calls.details}.vrm`,
          method: "GET",
          headers: {},
          expires_at: "2099-01-01T00:00:00Z",
        },
      });
    }
    return ok({ items: [], pagination: { total: 0, limit: 20, offset: 0 } });
  });
  return calls;
}

/** Leaves the room the way a Candidate does: Leave, confirm, and the room opens the report. */
export async function leaveRoom(page: Page, name = "Leave") {
  await page.getByRole("button", { name, exact: true }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: /Leave and submit|Rời phòng và nộp bài/ })
    .click();
  await expect(page).toHaveURL(`/reports/${roomId}`);
}
