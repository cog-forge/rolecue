import { describe, expect, it } from "vitest";
import { routes } from "@/config/routes";

describe("routes", () => {
  it("builds the recruiter application detail path", () => {
    expect(
      routes.recruiter.application(
        "33333333-3333-4333-8333-333333333333",
        "44444444-4444-4444-8444-444444444444",
      ),
    ).toBe(
      "/recruiter/job-postings/33333333-3333-4333-8333-333333333333/applications/44444444-4444-4444-8444-444444444444",
    );
  });

  it("builds the Admin interview detail path", () => {
    expect(routes.admin.interview("a/b")).toBe("/admin/interviews/a%2Fb");
  });

  it("encodes unsafe characters in both ids", () => {
    expect(routes.recruiter.application("a/b?c#d e", "x/y?z#w %")).toBe(
      "/recruiter/job-postings/a%2Fb%3Fc%23d%20e/applications/x%2Fy%3Fz%23w%20%25",
    );
  });
});
