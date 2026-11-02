import { expect, test } from "bun:test";
test("Vitest unit and component suite", async () => {
  const child = Bun.spawn(["bun", "run", "test"], {
    stdout: "inherit",
    stderr: "inherit",
  });
  expect(await child.exited).toBe(0);
}, 300_000);
