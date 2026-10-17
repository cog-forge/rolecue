import { describe, expect, it } from "vitest";
import {
  defaultListFilters,
  listApiParams,
  listSearch,
  parseListFilters,
} from "@/features/recruiter/job-postings/utils/list-filters";

describe("list filters", () => {
  it("reads valid URL values and falls back on anything else", () => {
    expect(
      parseListFilters({ status: "rejected", q: " Go ", page: "3" }),
    ).toEqual({
      status: "rejected",
      search: "Go",
      page: 3,
    });
    expect(
      parseListFilters({ status: "archived", page: "-1", q: ["a", "b"] }),
    ).toEqual(defaultListFilters);
    expect(parseListFilters({ q: "x".repeat(201) }).search).toBe("");
    // The offset Go receives must fit in an int32.
    expect(parseListFilters({ page: "107374183" }).page).toBe(107374183);
    expect(parseListFilters({ page: "107374184" }).page).toBe(1);
  });

  it("keeps links short and round-trips", () => {
    expect(listSearch(defaultListFilters)).toBe("");
    const filters = { status: "open" as const, search: "C# dev", page: 2 };
    const query = listSearch(filters);
    expect(query).toBe("?status=open&q=C%23+dev&page=2");
    expect(
      parseListFilters(Object.fromEntries(new URLSearchParams(query))),
    ).toEqual(filters);
  });

  it("builds the Go query, leaving out the all status and an empty search", () => {
    expect(listApiParams(defaultListFilters)).toEqual({ limit: 20, offset: 0 });
    expect(listApiParams({ status: "pending", search: "Go", page: 3 })).toEqual(
      {
        limit: 20,
        offset: 40,
        status: "pending",
        search: "Go",
      },
    );
  });
});
