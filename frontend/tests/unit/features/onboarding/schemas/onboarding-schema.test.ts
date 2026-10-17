import { expect, it } from "vitest";
import { createOnboardingSchema } from "@/features/onboarding/schemas/onboarding-schema";
import {
  profileValues,
  type Profile,
} from "@/features/profile/schemas/profile-schema";

const recruiter: Profile = {
  id: "11111111-1111-4111-8111-111111111111",
  full_name: "Alex",
  email: "alex@example.com",
  image: null,
  role: "recruiter",
  email_verified: true,
  company_name: null,
  company_website: null,
  created_at: "2026-10-08T00:00:00Z",
  updated_at: "2026-10-08T00:00:00Z",
};

it.each([
  ["rolecue.com", "https://rolecue.com"],
  ["  rolecue.com/jobs  ", "https://rolecue.com/jobs"],
  ["https://rolecue.com", "https://rolecue.com"],
])("normalizes %s before website validation", (input, normalized) => {
  const values = createOnboardingSchema(recruiter).parse({
    ...profileValues(recruiter),
    company_name: "RoleCue",
    company_website: input,
  });
  expect(values.company_website).toBe(normalized);
});

it.each([
  "",
  "http://rolecue.com",
  "https://user:password@rolecue.com",
  "https://",
  "role cue.com",
])("still rejects invalid website %s", (website) => {
  expect(
    createOnboardingSchema(recruiter).safeParse({
      ...profileValues(recruiter),
      company_name: "RoleCue",
      company_website: website,
    }).success,
  ).toBe(false);
});
