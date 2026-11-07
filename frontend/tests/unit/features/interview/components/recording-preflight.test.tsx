import { describe, expect, it, vi } from "vitest";
import { fireEvent, screen } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { RecordingPreflight } from "@/features/interview/components/recording-preflight";
import { CONSENT_VERSION } from "@/features/interview/recording/recording-consent";

// The camera is not under test here: a camera that never starts leaves the consent box on its own.
vi.mock("@/features/interview/video/use-camera", () => ({
  useCamera: () => ({
    snapshot: {
      status: "idle",
      stream: null,
      code: null,
      requestPending: false,
    },
    start: vi.fn(),
    stop: vi.fn(),
  }),
}));

describe("the recording consent box of the preflight", () => {
  it("starts unticked, shows the versioned text and explains what declining means", () => {
    const onConsentChange = vi.fn();
    renderWithIntl(<RecordingPreflight onConsentChange={onConsentChange} />);
    const box = screen.getByRole("checkbox", {
      name: /I consent to this interview being recorded/,
    });
    expect(box).not.toBeChecked();
    expect(CONSENT_VERSION).toBe("2026-11-v1");
    expect(
      screen.getByText(`Consent text version ${CONSENT_VERSION}`),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Without your consent the interview cannot start/),
    ).toBeInTheDocument();
    expect(onConsentChange).not.toHaveBeenCalled();
  });

  it("reports the tick and the untick", () => {
    const onConsentChange = vi.fn();
    renderWithIntl(<RecordingPreflight onConsentChange={onConsentChange} />);
    const box = screen.getByRole("checkbox");
    fireEvent.click(box);
    expect(box).toBeChecked();
    expect(onConsentChange).toHaveBeenLastCalledWith(true);
    expect(
      screen.queryByText(/Without your consent the interview cannot start/),
    ).not.toBeInTheDocument();
    fireEvent.click(box);
    expect(box).not.toBeChecked();
    expect(onConsentChange).toHaveBeenLastCalledWith(false);
  });

  it("shows the text in Vietnamese", () => {
    renderWithIntl(<RecordingPreflight />, { locale: "vi" });
    expect(
      screen.getByRole("checkbox", { name: /Tôi đồng ý cho buổi phỏng vấn/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(`Phiên bản nội dung đồng ý ${CONSENT_VERSION}`),
    ).toBeInTheDocument();
  });
});
