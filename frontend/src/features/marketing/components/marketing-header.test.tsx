import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { MarketingHeader } from "./marketing-header";

function setScrollPosition(value: number) {
  Object.defineProperty(window, "scrollY", {
    configurable: true,
    value,
    writable: true,
  });
}

beforeEach(() => {
  window.localStorage.clear();
  setScrollPosition(0);
  vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => {
    callback(0);
    return 0;
  });
  vi.stubGlobal("matchMedia", () => ({
    addEventListener: vi.fn(),
    matches: false,
    removeEventListener: vi.fn(),
  }));
});

afterEach(() => {
  vi.unstubAllGlobals();
});

it("opens and closes the mobile navigation with its disclosure state", () => {
  render(<MarketingHeader />);

  const trigger = screen.getByRole("button", { name: "Open navigation" });
  const mobileNavigation = document.getElementById("rolecue-mobile-navigation");
  if (!mobileNavigation) throw new Error("The mobile navigation is missing.");

  fireEvent.click(trigger);
  expect(trigger).toHaveAttribute("aria-expanded", "true");
  expect(mobileNavigation).toHaveAttribute("aria-hidden", "false");

  fireEvent.keyDown(window, { key: "Escape" });
  expect(trigger).toHaveAttribute("aria-expanded", "false");
  expect(trigger).toHaveFocus();
});

it("closes the mobile navigation when an outside interaction occurs", () => {
  render(<MarketingHeader />);

  const trigger = screen.getByRole("button", { name: "Open navigation" });
  fireEvent.click(trigger);
  fireEvent.pointerDown(document.body);

  expect(trigger).toHaveAttribute("aria-expanded", "false");
  expect(trigger).toHaveFocus();
});

it("closes the mobile navigation after a menu link is chosen", () => {
  render(<MarketingHeader />);

  const trigger = screen.getByRole("button", { name: "Open navigation" });
  const mobileNavigation = document.getElementById("rolecue-mobile-navigation");
  if (!mobileNavigation) throw new Error("The mobile navigation is missing.");

  fireEvent.click(trigger);
  const firstLink = mobileNavigation.querySelector("a");
  if (!firstLink) throw new Error("The mobile navigation has no links.");
  fireEvent.click(firstLink);

  expect(trigger).toHaveAttribute("aria-expanded", "false");
  expect(mobileNavigation).toHaveAttribute("aria-hidden", "true");
});

it("uses the observed scroll hysteresis for the condensed state", () => {
  render(<MarketingHeader />);

  const navigation = screen.getByRole("navigation", {
    name: "Main navigation",
  });

  setScrollPosition(65);
  fireEvent.scroll(window);
  expect(navigation).toHaveAttribute("data-condensed", "true");

  setScrollPosition(24);
  fireEvent.scroll(window);
  expect(navigation).toHaveAttribute("data-condensed", "true");

  setScrollPosition(23);
  fireEvent.scroll(window);
  expect(navigation).toHaveAttribute("data-condensed", "false");
});

it("dismisses the campaign without leaving its controls in the tab order", () => {
  render(<MarketingHeader />);

  fireEvent.click(screen.getByRole("button", { name: "Dismiss announcement" }));

  const campaign = screen.getByText("RoleCue practice is ready").parentElement
    ?.parentElement;
  expect(campaign).toHaveAttribute("aria-hidden", "true");
  expect(
    screen.getByRole("button", { name: "Dismiss announcement", hidden: true }),
  ).toHaveAttribute("tabindex", "-1");
});
