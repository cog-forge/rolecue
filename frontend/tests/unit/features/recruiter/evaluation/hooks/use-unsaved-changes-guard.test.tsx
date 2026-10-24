import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { useUnsavedChangesGuard } from "@/features/recruiter/evaluation/hooks/use-unsaved-changes-guard";

const mocks = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn() }));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mocks.push, replace: mocks.replace }),
}));

function Harness({ active }: { active: boolean }) {
  const guard = useUnsavedChangesGuard(active);
  return (
    <div>
      <a href="/recruiter/applications?status=pending">Applications</a>
      <a href="/recruiter/applications" target="_blank">
        New tab
      </a>
      <a href="/recruiter/applications" download>
        Download
      </a>
      <a href="https://example.com/">Other site</a>
      <a href="#weights">Same page</a>
      <p>{guard.leaving ? "asking" : "idle"}</p>
      <button onClick={guard.stay}>Stay</button>
      <button onClick={guard.leave}>Leave</button>
    </div>
  );
}

const link = (name: string) => screen.getByRole("link", { name });
const isGuardEntry = () =>
  Boolean(
    (window.history.state as Record<string, unknown> | null)?.[
      "__rolecueUnsavedChanges"
    ],
  );
// Keeps jsdom from trying to load a link the guard lets through.
const stopLoad = (event: Event) => event.preventDefault();

beforeEach(() => {
  vi.clearAllMocks();
  document.addEventListener("click", stopLoad);
});
afterEach(async () => {
  document.removeEventListener("click", stopLoad);
  cleanup();
  // An unmounted guard drops its entry with an asynchronous Back; let it land before the next test.
  await waitFor(() => expect(isGuardEntry()).toBe(false));
});

describe("useUnsavedChangesGuard", () => {
  it("stays out of the way while there is nothing to lose", () => {
    const entries = window.history.length;
    render(<Harness active={false} />);
    fireEvent.click(link("Applications"));
    expect(screen.getByText("idle")).toBeInTheDocument();
    const unload = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(unload);
    expect(unload.defaultPrevented).toBe(false);
    expect(window.history.length).toBe(entries);
  });

  it("asks before an in-app link and goes there only when the user leaves", () => {
    render(<Harness active />);
    expect(isGuardEntry()).toBe(true);
    const unload = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(unload);
    expect(unload.defaultPrevented).toBe(true);

    fireEvent.click(link("Applications"));
    expect(screen.getByText("asking")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Stay" }));
    expect(screen.getByText("idle")).toBeInTheDocument();
    expect(mocks.replace).not.toHaveBeenCalled();

    fireEvent.click(link("Applications"));
    fireEvent.click(screen.getByRole("button", { name: "Leave" }));
    // The guard's extra entry is on top, so it is replaced rather than left behind.
    expect(mocks.replace).toHaveBeenCalledWith(
      "/recruiter/applications?status=pending",
    );
    expect(mocks.push).not.toHaveBeenCalled();
    expect(screen.getByText("idle")).toBeInTheDocument();
  });

  it("lets through clicks that do not leave the page inside the app", () => {
    render(<Harness active />);
    fireEvent.click(link("Applications"), { ctrlKey: true });
    fireEvent.click(link("Applications"), { button: 1 });
    fireEvent.click(link("New tab"));
    fireEvent.click(link("Download"));
    fireEvent.click(link("Other site"));
    fireEvent.click(link("Same page"));
    expect(screen.getByText("idle")).toBeInTheDocument();
  });

  it("asks when the browser's Back button is pressed and restores the guard on stay", async () => {
    render(<Harness active />);
    expect(isGuardEntry()).toBe(true);
    window.history.back();
    await waitFor(() => expect(screen.getByText("asking")).toBeInTheDocument());
    expect(isGuardEntry()).toBe(false);
    fireEvent.click(screen.getByRole("button", { name: "Stay" }));
    expect(isGuardEntry()).toBe(true);
    expect(screen.getByText("idle")).toBeInTheDocument();
  });

  it("goes back for real when the user leaves after pressing Back", async () => {
    render(<Harness active />);
    window.history.back();
    await waitFor(() => expect(screen.getByText("asking")).toBeInTheDocument());
    const back = vi.spyOn(window.history, "back").mockImplementation(() => {});
    fireEvent.click(screen.getByRole("button", { name: "Leave" }));
    expect(back).toHaveBeenCalledTimes(1);
    // Released: the navigation that follows is not asked about again.
    const unload = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(unload);
    expect(unload.defaultPrevented).toBe(false);
  });

  it("removes its history entry once there is nothing to lose any more", async () => {
    const { rerender } = render(<Harness active />);
    expect(isGuardEntry()).toBe(true);
    rerender(<Harness active={false} />);
    await waitFor(() => expect(isGuardEntry()).toBe(false));
    fireEvent.click(link("Applications"));
    expect(screen.getByText("idle")).toBeInTheDocument();
  });
});
