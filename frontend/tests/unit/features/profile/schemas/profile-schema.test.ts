import { describe, expect, it } from "vitest";
import {
  createProfileFormSchema,
  profilePatch,
  profileValues,
  type Profile,
} from "@/features/profile/schemas/profile-schema";
const profile: Profile = {
  id: "11111111-1111-4111-8111-111111111111",
  full_name: "Nam",
  email: "nam@example.com",
  image: null,
  role: "candidate",
  email_verified: true,
  company_name: null,
  company_website: null,
  created_at: "2026-10-07T16:00:00Z",
  updated_at: "2026-10-07T16:00:00Z",
};
describe("profile contract", () => {
  it("validates Unicode name length and HTTPS avatar", () => {
    const schema = createProfileFormSchema(profile);
    expect(
      schema.safeParse({
        ...profileValues(profile),
        full_name: "ệ".repeat(100),
      }).success,
    ).toBe(true);
    expect(
      schema.safeParse({
        ...profileValues(profile),
        full_name: "ệ".repeat(101),
      }).success,
    ).toBe(false);
    expect(
      schema.safeParse({
        ...profileValues(profile),
        image: "http://example.com/a",
      }).success,
    ).toBe(false);
    expect(
      schema.safeParse({
        ...profileValues(profile),
        image: "https://u:p@example.com/a",
      }).success,
    ).toBe(false);
  });
  it("sends only changed allowed fields, with null for removed avatar", () => {
    const p = { ...profile, image: "https://example.com/a.png" };
    expect(profilePatch({ ...profileValues(p), image: "" }, p)).toEqual({
      image: null,
    });
    expect(
      profilePatch(
        {
          ...profileValues(p),
          full_name: "  New name  ",
          company_name: "forbidden",
        },
        p,
      ),
    ).toEqual({ full_name: "New name" });
  });
  it("allows saving a name without rejecting an unchanged legacy image", () => {
    const p = { ...profile, image: "http://legacy.example.com/a.png" };
    expect(
      createProfileFormSchema(p).safeParse({
        ...profileValues(p),
        full_name: "New name",
      }).success,
    ).toBe(true);
    expect(
      profilePatch({ ...profileValues(p), full_name: "New name" }, p),
    ).toEqual({ full_name: "New name" });
  });
  it("normalizes recruiter blanks to null and does not overwrite omitted fields", () => {
    const p = {
      ...profile,
      role: "recruiter" as const,
      company_name: "Acme",
      company_website: "https://acme.example",
    };
    expect(
      profilePatch({ ...profileValues(p), company_name: "  " }, p),
    ).toEqual({ company_name: null });
    expect(profilePatch(profileValues(p), p)).toEqual({});
  });
});
