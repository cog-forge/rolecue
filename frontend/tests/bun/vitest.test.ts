import { expect, test } from "bun:test";
// This timeout covers the entire Vitest suite, including each jsdom worker's startup.
// Keep per-test timeouts in Vitest unchanged while allowing slower CI runners to finish.
test("Vitest unit and component suite", async () => {
  const child = Bun.spawn(["bun", "run", "test"], {
    stdout: "inherit",
    stderr: "inherit",
  });
  expect(await child.exited).toBe(0);
}, 300_000);
