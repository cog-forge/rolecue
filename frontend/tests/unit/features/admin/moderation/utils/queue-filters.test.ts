import { describe, expect, it } from "vitest";
import {
  defaultQueueFilters,
  parseQueueFilters,
  queueApiParams,
  queueSearch,
} from "@/features/admin/moderation/utils/queue-filters";

const recruiterId = "22222222-2222-4222-8222-222222222222";

describe("parseQueueFilters", () => {
  it("defaults to the first page of the pending queue", () => {
    expect(parseQueueFilters({})).toEqual(defaultQueueFilters);
  });

  it("keeps valid filters", () => {
    expect(
      parseQueueFilters({
        status: "rejected",
        recruiter_id: recruiterId,
        from: "2026-10-01",
        to: "2026-10-31",
        page: "3",
      }),
    ).toEqual({
      status: "rejected",
      recruiterId,
      from: "2026-10-01",
      to: "2026-10-31",
      page: 3,
    });
    expect(parseQueueFilters({ status: "all" }).status).toBe("all");
  });

  it("drops malformed or repeated values instead of failing", () => {
    expect(
      parseQueueFilters({
        status: "archived",
        recruiter_id: "not-a-uuid",
        from: "2026-02-30",
        to: ["2026-10-01", "2026-10-02"],
        page: "-2",
      }),
    ).toEqual(defaultQueueFilters);
  });
});

describe("queueSearch", () => {
  it("leaves defaults out of the URL", () => {
    expect(queueSearch(defaultQueueFilters)).toBe("");
    expect(queueSearch({ ...defaultQueueFilters, status: "all", page: 2 })).toBe(
      "?status=all&page=2",
    );
  });

  it("round-trips through parseQueueFilters", () => {
    const filters = {
      status: "open" as const,
      recruiterId,
      from: "2026-10-01",
      to: "2026-10-02",
      page: 4,
    };
    const query = Object.fromEntries(new URLSearchParams(queueSearch(filters)));
    expect(parseQueueFilters(query)).toEqual(filters);
  });
});

describe("queueApiParams", () => {
  it("maps page to limit and offset", () => {
    expect(queueApiParams({ ...defaultQueueFilters, page: 3 })).toEqual({
      status: "pending",
      limit: 20,
      offset: 40,
    });
  });

  it("sends RFC 3339 dates with an exclusive end the day after 'to'", () => {
    expect(
      queueApiParams({
        ...defaultQueueFilters,
        recruiterId,
        from: "2026-10-01",
        to: "2026-10-31",
      }),
    ).toMatchObject({
      recruiter_id: recruiterId,
      from: "2026-10-01T00:00:00Z",
      to: "2026-11-01T00:00:00Z",
    });
  });
});
