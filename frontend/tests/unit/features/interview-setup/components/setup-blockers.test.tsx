import { expect, it } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { SetupBlockers } from "@/features/interview-setup/components/setup-blockers";
import {
  practiceBlockerCodes,
  recruitmentBlockerCodes,
} from "@/features/interview-setup/lib/setup-blockers";
import en from "@/lib/i18n/messages/en";
import vi from "@/lib/i18n/messages/vi";
import { applicationId, profileId } from "../saved-setup-fixtures";

const practice = { mode: "practice", roleProfileId: profileId } as const;
const recruitment = { mode: "recruitment", applicationId } as const;
const messages = en.candidate.setup.blockers.messages;

it("has an English and a Vietnamese message for every blocker code", () => {
  for (const code of [...practiceBlockerCodes, ...recruitmentBlockerCodes]) {
    expect(messages[code], code).toBeTruthy();
    expect(vi.candidate.setup.blockers.messages[code], code).toBeTruthy();
  }
});

it("renders nothing without blockers", () => {
  const { container } = renderWithIntl(
    <SetupBlockers
      blockers={[]}
      target={practice}
      params={{ roleProfileId: profileId }}
    />,
  );
  expect(container).toBeEmptyDOMElement();
});

it("explains each practice blocker and links a field to its step", () => {
  const params = { roleProfileId: profileId };
  renderWithIntl(
    <SetupBlockers
      target={practice}
      params={params}
      blockers={[
        { code: "avatar_required", field: "avatar" },
        { code: "voice_unavailable", field: "voice" },
        { code: "environment_required", field: "environment" },
        { code: "question_bank_not_ready" },
      ]}
    />,
  );
  expect(screen.getByText(messages.avatar_required)).toBeInTheDocument();
  expect(screen.getByText(messages.voice_unavailable)).toBeInTheDocument();
  expect(screen.getByText(messages.environment_required)).toBeInTheDocument();
  expect(
    screen.getByText(messages.question_bank_not_ready),
  ).toBeInTheDocument();
  const base = `/interviews/new/interviewer?roleProfileId=${profileId}`;
  const links = screen.getAllByRole("link", { name: "Fix in setup" });
  expect(links.map((link) => link.getAttribute("href"))).toEqual([
    `${base}#setup-avatar`,
    `${base}#setup-voice`,
    `${base}#setup-environment`,
  ]);
  expect(
    screen.getByRole("link", { name: "Open Role Profile" }),
  ).toHaveAttribute("href", `/role-profiles/${profileId}`);
});

it("sends recruitment blockers to the application or the room picker, and never offers a locked field", () => {
  renderWithIntl(
    <SetupBlockers
      target={recruitment}
      params={{}}
      blockers={[
        { code: "interview_deadline_passed" },
        { code: "locked_avatar_unavailable", field: "avatar" },
        { code: "environment_required", field: "environment" },
      ]}
    />,
  );
  expect(
    screen.getByText(messages.locked_avatar_unavailable),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("link", { name: "Back to application" }),
  ).toHaveAttribute("href", `/applications/${applicationId}`);
  expect(screen.getByRole("link", { name: "Choose a room" })).toHaveAttribute(
    "href",
    "#setup-environment",
  );
  expect(screen.getAllByRole("link")).toHaveLength(2);
});

it("falls back to a generic line for a code it does not know", () => {
  renderWithIntl(
    <SetupBlockers
      target={practice}
      params={{}}
      blockers={[{ code: "brand_new" }]}
    />,
  );
  expect(screen.getByText(messages.unknown)).toBeInTheDocument();
});
