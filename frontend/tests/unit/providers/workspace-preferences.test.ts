import { describe, expect, it } from "vitest";
import { parseWorkspacePreferences } from "@/lib/stores/workspace-preferences";

describe("workspace preference boundary", () => {
  it.each([
    null,
    "",
    "{broken",
    "[]",
    "null",
    '"dark"',
    '{"theme":"unknown","collapsed":"yes"}',
  ])("uses safe defaults for %s", (raw) => {
    expect(parseWorkspacePreferences(raw)).toEqual({
      theme: "system",
      collapsed: null,
    });
  });
  it.each(["light", "dark", "system"])(
    "preserves the explicit %s choice",
    (theme) => {
      expect(parseWorkspacePreferences(JSON.stringify({ theme })).theme).toBe(
        theme,
      );
    },
  );
  it("restores explicit choices and ignores unrelated values", () => {
    expect(
      parseWorkspacePreferences(
        '{"theme":"dark","collapsed":false,"role":"admin"}',
      ),
    ).toEqual({ theme: "dark", collapsed: false });
    expect(parseWorkspacePreferences('{"collapsed":true}')).toEqual({
      theme: "system",
      collapsed: true,
    });
  });
});
