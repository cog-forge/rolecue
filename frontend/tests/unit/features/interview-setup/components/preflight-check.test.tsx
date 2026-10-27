import { StrictMode } from "react";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { PreflightCheck } from "@/features/interview-setup/components/preflight-check";
import { deferred, installMediaFakes } from "../../interview/audio/media-fakes";

let browser: ReturnType<typeof installMediaFakes>;
beforeEach(() => {
  browser = installMediaFakes();
});
afterEach(() => {
  vi.unstubAllGlobals();
});

it("does not access media on mount, including Strict Mode; checks real readiness on action", async () => {
  const view = render(
    <StrictMode>
      <PreflightCheck />
    </StrictMode>,
  );
  expect(browser.media.getUserMedia).not.toHaveBeenCalled();
  expect(screen.getByText("Microphone not tested")).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "Test microphone" }));
  await screen.findByText("Microphone ready");
  expect(browser.media.getUserMedia).toHaveBeenCalledTimes(1);
  expect(
    screen.getByRole("combobox", { name: "Microphone input" }),
  ).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "Mute microphone" }));
  expect(screen.getByText("Microphone muted")).toBeVisible();
  expect(browser.tracks[0].enabled).toBe(false);
  fireEvent.click(screen.getByRole("button", { name: "Unmute microphone" }));
  expect(screen.getByText("Microphone ready")).toBeVisible();
  view.unmount();
  expect(browser.tracks[0].readyState).toBe("ended");
});

it("explains permission denial and retries explicitly", async () => {
  browser.media.getUserMedia.mockRejectedValueOnce(
    new DOMException("blocked", "NotAllowedError"),
  );
  render(<PreflightCheck />);
  fireEvent.click(screen.getByRole("button", { name: "Test microphone" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(
    /browser permission/,
  );
  expect(browser.media.getUserMedia).toHaveBeenCalledTimes(1);
  fireEvent.click(screen.getByRole("button", { name: "Retry microphone" }));
  await screen.findByText("Microphone ready");
});

it("cancels a permission prompt, disables new requests until it settles, and releases the late stream", async () => {
  const request = deferred<MediaStream>();
  browser.media.getUserMedia.mockReturnValueOnce(request.promise);
  render(<PreflightCheck />);
  fireEvent.click(screen.getByRole("button", { name: "Test microphone" }));
  fireEvent.click(
    screen.getByRole("button", { name: "Cancel microphone check" }),
  );
  expect(
    screen.getByText(/close or dismiss the browser permission prompt/i),
  ).toBeVisible();
  await act(async () => request.resolve(browser.stream()));
  await waitFor(() =>
    expect(
      screen.getByRole("button", { name: "Test microphone" }),
    ).toBeEnabled(),
  );
  expect(browser.tracks[0].readyState).toBe("ended");
  expect(screen.queryByText("Microphone ready")).not.toBeInTheDocument();
});

it("shows device loss with recovery and no interview start action", async () => {
  render(<PreflightCheck />);
  fireEvent.click(screen.getByRole("button", { name: "Test microphone" }));
  await screen.findByText("Microphone ready");
  act(() => browser.tracks[0].end());
  expect(screen.getByRole("alert")).toHaveTextContent(/disconnected/);
  expect(
    screen.getByRole("button", { name: "Retry microphone" }),
  ).toBeEnabled();
  expect(
    screen.queryByRole("button", { name: /start interview/i }),
  ).not.toBeInTheDocument();
  expect(browser.media.getUserMedia).toHaveBeenCalledWith({
    audio: true,
    video: false,
  });
});
