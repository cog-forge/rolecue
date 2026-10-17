import { describe, it, expect, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { createOnboardingSchema } from "@/features/onboarding/schemas/onboarding-schema";
import { meSchema } from "@/features/auth/schemas/session-schema";
import { OnboardingForm } from "@/features/onboarding/components/onboarding-form";
import {
  profileValues,
  type Profile,
} from "@/features/profile/schemas/profile-schema";
vi.mock("@/features/onboarding/components/website-preview", () => ({
  WebsitePreview: () => <aside>Website preview</aside>,
}));
const profile: Profile = {
  id: "11111111-1111-4111-8111-111111111111",
  full_name: "New user",
  email: "alex@example.com",
  image: null,
  role: "candidate",
  email_verified: true,
  company_name: null,
  company_website: null,
  created_at: "2026-10-08T00:00:00Z",
  updated_at: "2026-10-08T00:00:00Z",
};
describe("onboarding metadata", () => {
  it("candidate accepts an empty image for Peek", () => {
    expect(
      createOnboardingSchema(profile).safeParse({
        ...profileValues(profile),
        full_name: "Alex",
        image: "",
      }).success,
    ).toBe(true);
  });
  it("requires recruiter company and HTTPS website", () => {
    const recruiter = { ...profile, role: "recruiter" as const };
    const values = { ...profileValues(recruiter), full_name: "Alex" };
    expect(createOnboardingSchema(recruiter).safeParse(values).success).toBe(
      false,
    );
    expect(
      createOnboardingSchema(recruiter).safeParse({
        ...values,
        company_name: "Acme",
        company_website: "https://acme.example",
      }).success,
    ).toBe(true);
  });
  it("requires explicit onboarding state in bootstrap", () => {
    expect(
      meSchema.safeParse({
        success: true,
        data: { ...profile, is_locked: false },
      }).success,
    ).toBe(false);
  });
  it("submits all mandatory candidate metadata without inventing an image", async () => {
    const save = vi.fn().mockResolvedValue(undefined);
    render(
      <OnboardingForm
        profile={profile}
        saving={false}
        error={null}
        onSave={save}
      />,
    );
    expect(screen.getByText(/Peek avatar is ready/)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Display name"), {
      target: { value: " Alex " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Open my workspace" }));
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith({ full_name: "Alex" }),
    );
  });
  it("keeps an unchanged OAuth image outside the metadata patch", async () => {
    const save = vi.fn().mockResolvedValue(undefined);
    render(
      <OnboardingForm
        profile={{
          ...profile,
          full_name: "Alex",
          image: "http://legacy.example/avatar.png",
        }}
        saving={false}
        error={null}
        onSave={save}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Open my workspace" }));
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith({ full_name: "Alex" }),
    );
  });
  it("recruiter cannot submit until website is supplied", async () => {
    const save = vi.fn();
    render(
      <OnboardingForm
        profile={{ ...profile, role: "recruiter" }}
        saving={false}
        error={null}
        onSave={save}
      />,
    );
    fireEvent.change(screen.getByLabelText("Display name"), {
      target: { value: "Alex" },
    });
    fireEvent.change(screen.getByLabelText("Company name"), {
      target: { value: "Acme" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Open my workspace" }));
    expect(
      await screen.findByText("Enter your company website."),
    ).toBeInTheDocument();
    expect(save).not.toHaveBeenCalled();
  });
});
