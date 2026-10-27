import { expect, type Page } from "@playwright/test";

export const cors = {
  "access-control-allow-origin": "http://127.0.0.1:3100",
  "access-control-allow-credentials": "true",
  "access-control-allow-methods": "GET,POST,DELETE,OPTIONS",
  "access-control-allow-headers": "content-type",
};

export type Role = "candidate" | "recruiter" | "admin";
export const savedAvatar = {
  id: "9b9e9289-994d-4e62-9e65-034b9a907331",
  name: "Alex",
  personality: "Supportive and precise",
  created_at: "2026-10-13T00:00:00Z",
  updated_at: "2026-10-13T00:00:00Z",
};

export const avatarItem = (n: number) => ({
  id: `1111111${n}-1111-4111-8111-111111111111`,
  name: `Avatar ${n}`,
  personality: `A calm interviewer number ${n} who asks follow-up questions.`,
  created_at: "2026-10-20T00:00:00Z",
  updated_at: "2026-10-20T00:00:00Z",
});

type Reply = { status: number; code?: string } | "abort";

// One mocked Go API for the avatar flow. It proves browser integration, not real conversion.
export async function mockAvatarApi(
  page: Page,
  options: {
    role: Role;
    capacity?: number;
    items?: ReturnType<typeof avatarItem>[];
    balance?: number;
    create?: Reply;
    remove?: Reply;
    purchase?: Reply;
    grant?: { url: string; expiresInMs: number };
  },
) {
  let capacity = options.capacity ?? 3;
  const items = [...(options.items ?? [])];
  const calls = {
    inventory: 0,
    creates: 0,
    deletes: 0,
    purchases: 0,
    details: 0,
  };
  const fail = (code: string, status: number) => ({
    status,
    headers: cors,
    json: { success: false, error: { code, message: code } },
  });

  await page.route("http://localhost:8080/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const method = request.method();
    if (method === "OPTIONS")
      return route.fulfill({ status: 204, headers: cors });
    const ok = (data: unknown, status = 200) =>
      route.fulfill({ status, headers: cors, json: { success: true, data } });
    const summary = () => ({
      capacity,
      used: items.length,
      available: Math.max(capacity - items.length, 0),
      is_full: items.length >= capacity,
    });

    if (url.pathname === "/auth/me")
      return ok({
        id: "9b9e9289-994d-4e62-9e65-034b9a907330",
        email: "alex@example.com",
        full_name: "Alex Doe",
        image: null,
        role: options.role,
        email_verified: true,
        is_locked: false,
        onboarding_role_selected: true,
        onboarding_completed: true,
      });
    if (url.pathname === "/inventory") {
      calls.inventory++;
      return ok(summary());
    }
    if (url.pathname === "/inventory/avatars" && method === "GET") {
      const limit = Number(url.searchParams.get("limit"));
      const offset = Number(url.searchParams.get("offset"));
      return ok({
        items: items.slice(offset, offset + limit),
        pagination: { limit, offset, total: items.length },
      });
    }
    if (url.pathname === "/inventory/avatars" && method === "POST") {
      calls.creates++;
      const body = request.postDataBuffer()?.toString() ?? "";
      expect(body).toContain('name="file"; filename="avatar.glb"');
      expect(body).toContain(
        JSON.stringify({
          name: savedAvatar.name,
          personality: savedAvatar.personality,
        }),
      );
      const reply = options.create ?? { status: 201 };
      if (reply === "abort") return route.abort("failed");
      if (reply.code) return route.fulfill(fail(reply.code, reply.status));
      items.unshift(savedAvatar);
      return ok(savedAvatar, 201);
    }
    const detail = url.pathname.match(/^\/inventory\/avatars\/([^/]+)$/);
    if (detail && method === "GET") {
      calls.details++;
      const avatar = items.find((item) => item.id === detail[1]);
      if (!avatar) return route.fulfill(fail("AVATAR_NOT_FOUND", 404));
      const grant = options.grant ?? {
        url: "https://s3.test/avatars/model.vrm",
        expiresInMs: 300_000,
      };
      return ok({
        ...avatar,
        model_access: {
          url: `${grant.url}?attempt=${calls.details}`,
          method: "GET",
          headers: {},
          expires_at: new Date(Date.now() + grant.expiresInMs).toISOString(),
        },
      });
    }
    if (detail && method === "DELETE") {
      calls.deletes++;
      const reply = options.remove ?? { status: 204 };
      if (reply === "abort") return route.abort("failed");
      if (reply.code) return route.fulfill(fail(reply.code, reply.status));
      items.splice(
        items.findIndex((item) => item.id === detail[1]),
        1,
      );
      return route.fulfill({ status: 204, headers: cors });
    }
    if (url.pathname === "/inventory/purchase") {
      calls.purchases++;
      const reply = options.purchase ?? { status: 200 };
      if (reply === "abort") return route.abort("failed");
      if (reply.code) return route.fulfill(fail(reply.code, reply.status));
      capacity += (request.postDataJSON() as { amount: number }).amount;
      return ok({ ...summary(), balance: (options.balance ?? 5) - 1 });
    }
    if (url.pathname === "/wallet")
      return ok({
        id: "wallet",
        user_id: "user",
        balance: options.balance ?? 5,
      });
    return route.fulfill(fail("NOT_MOCKED", 404));
  });
  return calls;
}
