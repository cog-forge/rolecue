import { describe, expect, it } from "vitest";
import {
  applicationApiParams,
  applicationSearch,
  defaultApplicationFilters,
  parseApplicationFilters,
} from "@/features/recruiter/applications/utils/application-filters";

const posting = "33333333-3333-4333-8333-333333333333";

describe("application filters", () => {
  it("reads valid URL values and falls back on anything else", () => {
    expect(
      parseApplicationFilters({
        status: "interview_eligible",
        q: " Linh ",
        page: "3",
        posting: posting.toUpperCase(),
      }),
    ).toEqual({
      status: "interview_eligible",
      search: "Linh",
      page: 3,
      posting,
    });
    expect(
      parseApplicationFilters({
        status: "hired",
        page: "-1",
        q: ["a", "b"],
        posting: "not-a-uuid",
      }),
    ).toEqual(defaultApplicationFilters);
    expect(parseApplicationFilters({ q: "x".repeat(201) }).search).toBe("");
    // The offset Go receives must fit in an int32.
    expect(parseApplicationFilters({ page: "107374183" }).page).toBe(107374183);
    expect(parseApplicationFilters({ page: "107374184" }).page).toBe(1);
  });

  it("accepts every Application status", () => {
    for (const status of [
      "pending",
      "interview_eligible",
      "interviewed",
      "approved",
      "rejected",
    ]) {
      expect(parseApplicationFilters({ status }).status).toBe(status);
    }
  });

  it("keeps links short and round-trips", () => {
    expect(applicationSearch(defaultApplicationFilters)).toBe("");
    const filters = {
      status: "rejected" as const,
      search: "a@b.co",
      page: 2,
      posting,
    };
    const query = applicationSearch(filters);
    expect(query).toBe(`?status=rejected&q=a%40b.co&posting=${posting}&page=2`);
    expect(
      parseApplicationFilters(Object.fromEntries(new URLSearchParams(query))),
    ).toEqual(filters);
  });

  it("builds the Go query, leaving out the all status and an empty search", () => {
    expect(applicationApiParams(defaultApplicationFilters, true)).toEqual({
      limit: 20,
      offset: 0,
    });
    const filters = {
      status: "pending" as const,
      search: "Linh",
      page: 3,
      posting,
    };
    expect(applicationApiParams(filters, true)).toEqual({
      limit: 20,
      offset: 40,
      status: "pending",
      q: "Linh",
      job_posting_id: posting,
    });
  });

  it("sends the posting filter only when asked, because a posting list takes it from the path", () => {
    const filters = { ...defaultApplicationFilters, posting };
    expect(applicationApiParams(filters, false)).not.toHaveProperty(
      "job_posting_id",
    );
  });
});
