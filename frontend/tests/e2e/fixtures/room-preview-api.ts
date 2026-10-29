import type { Page } from "@playwright/test";
import { avatarItem, cors } from "./avatar-api";
export const roomId = "aaaaaaaa-aaaa-4aaa-8aaa-000000000053";
export async function mockRoomPreview(
  page: Page,
  options: {
    recruitment?: boolean;
    avatar?: boolean;
    error?: boolean;
    completed?: boolean;
  } = {},
) {
  const calls = { reads: 0, details: 0, writes: [] as string[] };
  await page.route("http://localhost:8080/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (request.method() === "OPTIONS")
      return route.fulfill({ status: 204, headers: cors });
    const ok = (data: unknown) =>
      route.fulfill({ headers: cors, json: { success: true, data } });
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
