import type { Pool } from "pg";

export function createHandler(
  auth: { handler: (request: Request) => Promise<Response> },
  database: Pool,
) {
  return async (request: Request): Promise<Response> => {
    const { pathname } = new URL(request.url);
    const headers = { "Cache-Control": "no-store" };
    if (pathname === "/healthz")
      return Response.json({ status: "ok" }, { headers });
    if (pathname === "/readyz") {
      try {
        // Go migrations own the schema; auth only waits for a clean, applied state.
        const result = await database.query(
          "SELECT dirty FROM schema_migrations",
        );
        if (result.rows.length !== 1 || result.rows[0].dirty) throw new Error();
        return Response.json({ status: "ready" }, { headers });
      } catch {
        return Response.json(
          { status: "unavailable" },
          { status: 503, headers },
        );
      }
    }
    if (!pathname.startsWith("/api/auth/"))
      return new Response(null, { status: 404, headers });
    if (!["GET", "POST"].includes(request.method))
      return new Response(null, {
        status: 405,
        headers: { ...headers, Allow: "GET, POST" },
      });
    try {
      const response = await auth.handler(request);
      response.headers.set("Cache-Control", "no-store");
      // A database outage is availability failure, never an anonymous session.
      if (response.status >= 500) {
        const failure = new Headers(response.headers);
        failure.delete("Content-Length");
        failure.delete("Content-Type");
        return Response.json(
          { error: "Authentication service unavailable" },
          { status: 503, headers: failure },
        );
      }
      return response;
    } catch {
      // Never include URLs, provider payloads, request headers or exception text.
      console.error("Auth request unavailable");
      return Response.json(
        { error: "Authentication service unavailable" },
        { status: 503, headers },
      );
    }
  };
}
