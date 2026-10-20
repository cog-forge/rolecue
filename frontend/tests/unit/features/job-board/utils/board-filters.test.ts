import { describe, expect, it } from "vitest";
import {
  boardApiParams,
  boardSearch,
  defaultBoardFilters,
  parseBoardFilters,
} from "@/features/job-board/utils/board-filters";

describe("board filters", () => {
  it("defaults when the URL has nothing", () => {
    expect(parseBoardFilters({})).toEqual(defaultBoardFilters);
  });

  it("reads and trims search, location, type and page", () => {
    expect(
      parseBoardFilters({
        q: "  Go ",
        location: " Hanoi",
        type: "Full-time ",
        page: "3",
      }),
    ).toEqual({ search: "Go", location: "Hanoi", type: "Full-time", page: 3 });
  });

  it.each([
    ["0"],
    ["-1"],
    ["1.5"],
    ["abc"],
    ["107374184"], // offset would pass int32
  ])("falls back to page 1 for %s", (page) => {
    expect(parseBoardFilters({ page }).page).toBe(1);
  });

  it("keeps the last allowed page", () => {
    expect(parseBoardFilters({ page: "107374183" }).page).toBe(107374183);
  });

  it("ignores repeated params and over-long text", () => {
    const filters = parseBoardFilters({
      q: ["a", "b"],
      location: "x".repeat(201),
      type: "é".repeat(200),
    });
    expect(filters.search).toBe("");
    expect(filters.location).toBe("");
    expect(filters.type).toBe("é".repeat(200));
  });

  it("builds a short URL query that round-trips", () => {
    const filters = {
      search: "Go dev",
      location: "Hà Nội",
      type: "Full-time",
      page: 2,
    };
    const query = boardSearch(filters);
    expect(query).toBe(
      "?q=Go+dev&location=H%C3%A0+N%E1%BB%99i&type=Full-time&page=2",
    );
    expect(
      parseBoardFilters(Object.fromEntries(new URLSearchParams(query))),
    ).toEqual(filters);
    expect(boardSearch(defaultBoardFilters)).toBe("");
  });

  it("maps filters to the Go query names", () => {
    expect(boardApiParams(defaultBoardFilters)).toEqual({
      limit: 20,
      offset: 0,
    });
    expect(
      boardApiParams({
        search: "Go",
        location: "Hanoi",
        type: "Full-time",
        page: 3,
      }),
    ).toEqual({
      limit: 20,
      offset: 40,
      search: "Go",
      location: "Hanoi",
      employment_type: "Full-time",
    });
  });
});
