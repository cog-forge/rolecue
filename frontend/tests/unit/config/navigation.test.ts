import { describe, expect, it } from "vitest";
import {
  getActiveNavigation,
  workspaceNavigation,
  workspaceHome,
} from "@/config/navigation";

describe("workspace navigation", () => {
  it("resolves a distinct landing page for every validated role", () => {
    expect(workspaceHome("candidate")).toBe("/dashboard");
    expect(workspaceHome("recruiter")).toBe("/recruiter/dashboard");
    expect(workspaceHome("admin")).toBe("/admin/dashboard");
  });

  it("keeps recruitment, practice and governance destinations isolated", () => {
    const links = (role: "candidate" | "recruiter" | "admin") =>
      workspaceNavigation[role].flatMap((group) =>
        group.items.map((item) => item.href),
      );
    expect(
      links("recruiter").every((href) => href.startsWith("/recruiter/")),
    ).toBe(true);
    expect(links("candidate")).not.toContain("/admin/users");
    expect(links("admin")).not.toContain("/billing");
    expect(links("admin")).not.toContain("/admin/avatars");
    expect(links("admin")).not.toContain("/admin/questions");
  });

  it("matches segment boundaries, ignores query/hash, and maps supporting flows", () => {
    expect(
      getActiveNavigation("candidate", "/jobs/123?tab=details#apply")?.href,
    ).toBe("/jobs");
    expect(getActiveNavigation("candidate", "/jobs-other")).toBeUndefined();
    expect(
      getActiveNavigation("candidate", "/interviews/new/setup")?.href,
    ).toBe("/target-jds");
    expect(getActiveNavigation("candidate", "/reports/report-1")?.href).toBe(
      "/history",
    );
    expect(getActiveNavigation("admin", "/admin/users/123")?.href).toBe(
      "/admin/users",
    );
    expect(getActiveNavigation("admin", "/admin")).toBeUndefined();
    expect(getActiveNavigation("candidate", "/profile")).toBeUndefined();
  });
});
