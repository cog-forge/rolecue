import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { describe, expect, it } from "vitest";
import {
  findUnusedKeys,
  leafKeys,
  scanSource,
} from "@tests/helpers/catalog-usage";
import en from "@/lib/i18n/messages/en";
import vi_ from "@/lib/i18n/messages/vi";

// Keys that are only built at run time in a way the scanner cannot follow. Each entry needs the
// reason; prefer fixing the code so the key appears as a literal over adding one here.
const allowedUnused: { key: string; reason: string }[] = [
  {
    key: "common.globalError.title",
    reason:
      "global-error.tsx replaces the root layout, so it reads common.json directly instead of useTranslations",
  },
  {
    key: "common.globalError.description",
    reason: "same as common.globalError.title",
  },
];

const srcRoot = join(process.cwd(), "src");
const catalogDir = join("src", "lib", "i18n", "messages");

function sourceFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return sourceFiles(path);
    return /\.(ts|tsx)$/.test(name) && !/\.d\.ts$/.test(name) ? [path] : [];
  });
}

const scans = sourceFiles(srcRoot)
  .filter((file) => !relative(process.cwd(), file).startsWith(catalogDir))
  .map((file) => scanSource(file, readFileSync(file, "utf8")));

describe("catalog usage", () => {
  it("has no English key that no source file refers to", () => {
    const allowed = new Set(allowedUnused.map(({ key }) => key));
    const unused = findUnusedKeys(leafKeys(en), scans).filter(
      (key) => !allowed.has(key),
    );
    expect(unused).toEqual([]);
  });

  it("has no Vietnamese key that no source file refers to", () => {
    const allowed = new Set(allowedUnused.map(({ key }) => key));
    const unused = findUnusedKeys(leafKeys(vi_), scans).filter(
      (key) => !allowed.has(key),
    );
    expect(unused).toEqual([]);
  });

  it("does not allow a key that is already used or does not exist", () => {
    const keys = new Set(leafKeys(en));
    const unused = new Set(findUnusedKeys([...keys], scans));
    for (const { key, reason } of allowedUnused) {
      expect(reason).not.toBe("");
      expect(keys.has(key), `${key} is not in the catalog`).toBe(true);
      expect(unused.has(key), `${key} is used, drop the allowlist entry`).toBe(
        true,
      );
    }
  });
});

describe("scanSource", () => {
  const unusedAmong = (keys: string[], source: string) =>
    findUnusedKeys(keys, [scanSource("file.tsx", source)]);

  it("counts a literal key on a translator bound to a namespace", () => {
    const source = `
      const t = useTranslations("area.screen");
      export const a = t("title");
      export const b = t.rich("body", {});
    `;
    expect(
      unusedAmong(
        ["area.screen.title", "area.screen.body", "area.screen.other"],
        source,
      ),
    ).toEqual(["area.screen.other"]);
  });

  it("follows getTranslations, awaited and with an options object", () => {
    const source = `
      const a = await getTranslations("one");
      const b = await getTranslations({ locale, namespace: "two.deep" });
      a("x"); b("y");
    `;
    expect(unusedAmong(["one.x", "two.deep.y", "two.deep.z"], source)).toEqual([
      "two.deep.z",
    ]);
  });

  it("counts every key under a template-literal head", () => {
    const source = `
      const t = useTranslations("area");
      export const label = (s: string) => t(\`status.\${s}\`);
    `;
    expect(
      unusedAmong(
        ["area.status.open", "area.status.closed", "area.other.open"],
        source,
      ),
    ).toEqual(["area.other.open"]);
  });

  it("counts a literal held in a map when the translator is called by another file's code", () => {
    const source = `
      const items = [{ label: "home" }, { label: "billing" }];
      const t = useTranslations("nav");
      export const heading = t("title");
    `;
    expect(
      unusedAmong(["nav.home", "nav.billing", "nav.title", "nav.gone"], source),
    ).toEqual(["nav.gone"]);
  });

  it("counts a key passed to a translator received as a parameter", () => {
    const source = `
      export function message(t: Translator, code: string) {
        return t("emailRequired") ?? t(\`api.\${code}\`);
      }
    `;
    expect(
      unusedAmong(
        ["validation.emailRequired", "errors.api.NOT_FOUND", "errors.gone"],
        source,
      ),
    ).toEqual(["errors.gone"]);
  });

  it("uses the namespace in a translator type for helpers", () => {
    const source = `
      type T = ReturnType<typeof useTranslations<"area.errors">>;
      export const pick = (t: T, code: "A" | "B") => t(code === "A" ? "A" : "B");
    `;
    expect(
      unusedAmong(["area.errors.A", "area.errors.B", "area.c"], source),
    ).toEqual(["area.c"]);
  });

  it("counts a group named by its full path", () => {
    expect(
      unusedAmong(
        ["common.format.timeLeft.hours", "common.format.x"],
        `
        const group = "common.format.timeLeft";
      `,
      ),
    ).toEqual(["common.format.x"]);
  });

  it("does not count a key that appears only in an unrelated string", () => {
    const source = `
      const t = useTranslations("area");
      export const css = "flex gap-2";
      export const a = t("used");
    `;
    expect(unusedAmong(["area.used", "area.unused"], source)).toEqual([
      "area.unused",
    ]);
  });
});

describe("scanSource with opaque arguments", () => {
  it("treats the keys directly under a namespace as used when the key is a variable", () => {
    const source = `
      const t = useTranslations("area.status");
      export const label = (status: Status) => t(status);
    `;
    expect(
      findUnusedKeys(
        [
          "area.status.open",
          "area.status.closed",
          "area.status.deep.x",
          "area.other",
        ],
        [scanSource("file.tsx", source)],
      ),
    ).toEqual(["area.status.deep.x", "area.other"]);
  });

  it("reads both branches of a conditional key", () => {
    const source = `
      const t = useTranslations("area");
      export const label = (on: boolean) => t(on ? "yes" : "no");
    `;
    expect(
      findUnusedKeys(
        ["area.yes", "area.no", "area.maybe"],
        [scanSource("f.tsx", source)],
      ),
    ).toEqual(["area.maybe"]);
  });
});
