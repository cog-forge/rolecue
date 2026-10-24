import { Pool } from "pg";
import { createAuth } from "./auth";
import { readEnv } from "./env";
import { createHandler } from "./handler";

try {
  const config = readEnv();
  const pool = new Pool({
    connectionString: config.connectionString,
    max: 10,
    connectionTimeoutMillis: 5000,
    query_timeout: 5000,
    statement_timeout: 5000,
  });
  pool.on("error", () => console.error("Auth database connection unavailable"));
  const auth = createAuth(config, pool);
  const server = Bun.serve({
    port: config.port,
    hostname: config.hostname,
    maxRequestBodySize: 1024 * 1024,
    idleTimeout: 30,
    fetch: createHandler(auth, pool),
  });
  console.info(`Auth service listening on port ${config.port}`);
  let stopping = false;
  const shutdown = async () => {
    if (stopping) return;
    stopping = true;
    const deadline = setTimeout(() => process.exit(1), 10_000);
    deadline.unref();
    await server.stop();
    await pool.end();
    clearTimeout(deadline);
  };
  process.on("SIGTERM", shutdown);
  process.on("SIGINT", shutdown);
} catch {
  console.error("Auth startup failed: check runtime configuration");
  process.exitCode = 1;
}
