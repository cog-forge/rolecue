import { stat } from "node:fs/promises";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { environmentScenes } from "@/components/avatar-viewer/environment/scenes";

const size = async (path: string) =>
  (await stat(join(process.cwd(), "public", path))).size;

describe("environment scenes", () => {
  it.each(Object.entries(environmentScenes))(
    "%s ships its files and reports their real size for the progress bar, within the 8 MB budget",
    async (_, scene) => {
      const bytes =
        (await size(scene.backplate)) +
        (await size(scene.lighting)) +
        (await size(scene.props));
      expect(scene.bytes).toBe(bytes);
      expect(bytes).toBeLessThanOrEqual(8 * 1024 * 1024);
      expect(await size(scene.still)).toBeGreaterThan(0);
      expect(await size(scene.thumbnail)).toBeGreaterThan(0);
    },
  );
});
