import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { fireEvent, screen } from "@testing-library/react";
import { createTranslator } from "next-intl";
import { messagesFor, renderWithIntl } from "@tests/helpers/intl";
import { PreflightCheck } from "@/features/interview-setup/components/preflight-check";
import {
  microphoneMessage,
  microphoneMessageKeys,
} from "@/features/interview-setup/lib/microphone-messages";
import { installMediaFakes } from "../../interview/audio/media-fakes";

let browser: ReturnType<typeof installMediaFakes>;
beforeEach(() => {
  browser = installMediaFakes();
});
afterEach(() => {
  vi.unstubAllGlobals();
});

it("checks the microphone in Vietnamese from idle to ready and muted", async () => {
  renderWithIntl(<PreflightCheck />, { locale: "vi" });
  expect(
    screen.getByRole("heading", { name: "Kiểm tra micro của bạn" }),
  ).toBeVisible();
  expect(screen.getByText("Chưa kiểm tra micro")).toBeVisible();
  expect(screen.getByText("Mức âm đầu vào chưa hoạt động")).toBeVisible();
  expect(screen.getByRole("combobox", { name: "Đầu vào micro" })).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "Kiểm tra micro" }));
  await screen.findByText("Micro đã sẵn sàng");
  fireEvent.click(screen.getByRole("button", { name: "Tắt tiếng micro" }));
  expect(screen.getByText("Micro đang tắt tiếng")).toBeVisible();
  expect(screen.getByRole("button", { name: "Bật tiếng micro" })).toBeVisible();
  expect(
    screen.getByRole("meter", { name: "Mức âm đầu vào của micro" }),
  ).toBeVisible();
});

it("translates a permission failure shown before the room", async () => {
  browser.media.getUserMedia.mockRejectedValueOnce(
    new DOMException("blocked", "NotAllowedError"),
  );
  renderWithIntl(<PreflightCheck />, { locale: "vi" });
  fireEvent.click(screen.getByRole("button", { name: "Kiểm tra micro" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "Chưa được cho phép truy cập micro.",
  );
  expect(screen.getByRole("button", { name: "Thử lại micro" })).toBeVisible();
});

it("translates the capability list and keeps the English names byte for byte", () => {
  renderWithIntl(<PreflightCheck />, { locale: "vi" });
  fireEvent.click(
    screen.getByRole("button", { name: "Kiểm tra tính năng của trình duyệt" }),
  );
  expect(screen.getByText("Web Audio")).toBeVisible();
  expect(screen.getAllByText(/Có sẵn|Không có sẵn/)).toHaveLength(6);
});

it("maps every known controller sentence to its translation and passes unknown ones through", () => {
  const sources = [
    "features/interview/audio/microphone-capture.ts",
    "features/interview/audio/audio-monitor.ts",
  ].map((file) => readFileSync(resolve("src", file), "utf8"));
  for (const sentence of Object.keys(microphoneMessageKeys))
    // The sentences are source literals, possibly wrapped across lines.
    expect(
      sources.some((source) =>
        source.replace(/"\s*\n\s*"/g, "").includes(sentence),
      ),
      sentence,
    ).toBe(true);
  const t = (locale: "en" | "vi") =>
    createTranslator({
      locale,
      messages: messagesFor(locale),
      namespace: "candidate.setup.preflight",
    });
  for (const sentence of Object.keys(microphoneMessageKeys)) {
    expect(microphoneMessage(sentence, t("en"))).toBe(sentence);
    expect(microphoneMessage(sentence, t("vi"))).not.toBe(sentence);
  }
  expect(microphoneMessage("Something new", t("vi"))).toBe("Something new");
});
