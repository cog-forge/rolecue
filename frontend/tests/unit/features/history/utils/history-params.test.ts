import { describe, expect, it } from "vitest";
import {
  historyHref,
  parseHistoryFilters,
} from "@/features/history/utils/history-params";

describe("parseHistoryFilters", () => {
  it("defaults to every type on page 1", () => {
    expect(parseHistoryFilters(new URLSearchParams())).toEqual({
      type: undefined,
      page: 1,
    });
  });

  it("reads a known type and page from the URL", () => {
    expect(
      parseHistoryFilters(new URLSearchParams("type=recruitment&page=3")),
    ).toEqual({ type: "recruitment", page: 3 });
  });

  it.each(["0", "-2", "abc", "1.5", ""])(
    "falls back to page 1 for page %j",
    (page) => {
      expect(parseHistoryFilters(new URLSearchParams({ page }))).toEqual({
        type: undefined,
        page: 1,
      });
    },
  );

  it("ignores an unknown type instead of sending it to the API", () => {
    expect(parseHistoryFilters(new URLSearchParams("type=questions"))).toEqual({
      type: undefined,
      page: 1,
    });
  });
});

describe("historyHref", () => {
  it("keeps the plain history address for the default view", () => {
    expect(historyHref({ type: undefined, page: 1 })).toBe("/history");
  });

  it("adds only the parameters that are set", () => {
    expect(historyHref({ type: "practice", page: 1 })).toBe(
      "/history?type=practice",
    );
    expect(historyHref({ type: undefined, page: 2 })).toBe("/history?page=2");
    expect(historyHref({ type: "recruitment", page: 4 })).toBe(
      "/history?type=recruitment&page=4",
    );
  });
});
