import { useEffect } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import {
  AvatarViewer,
  type ModelAccess,
} from "@/components/avatar-viewer/avatar-viewer";
import type { CanvasStatus } from "@/components/avatar-viewer/avatar-canvas";

const mocks = vi.hoisted(() => ({
  webgl: true,
  mounts: 0,
  crash: false,
  report: null as ((status: CanvasStatus) => void) | null,
}));
vi.mock("@/components/avatar-viewer/webgl", () => ({
  isWebGLAvailable: () => mocks.webgl,
}));
// The real canvas needs WebGL; this stand-in lets each test drive the load result.
vi.mock("@/components/avatar-viewer/avatar-canvas", () => ({
  default: function FakeCanvas({
    access,
    onStatus,
  }: {
    access: ModelAccess;
    onStatus: (status: CanvasStatus) => void;
  }) {
    if (mocks.crash) throw new Error("render failed");
    useEffect(() => {
      mocks.mounts += 1;
      mocks.report = onStatus;
      onStatus({ state: "loading" });
    }, [access, onStatus]);
    return <canvas data-testid="canvas" data-url={access.url} />;
  },
}));

const grant = (url: string): ModelAccess => ({
  url,
  method: "GET",
  headers: {},
  expiresAt: "2026-10-27T10:05:00Z",
});

function report(status: CanvasStatus) {
  act(() => mocks.report?.(status));
}

beforeEach(() => {
  mocks.webgl = true;
  mocks.mounts = 0;
  mocks.crash = false;
  mocks.report = null;
});

describe("AvatarViewer", () => {
  it("shows an explicit no-asset fallback while preserving null-access loading for existing callers", () => {
    const { rerender } = render(
      <AvatarViewer
        access={null}
        label="Interviewer"
        onAccessExpired={vi.fn()}
      />,
    );
    expect(screen.getByRole("status")).toHaveTextContent("Loading 3D preview");
    rerender(
      <AvatarViewer
        access={null}
        label="Interviewer"
        onAccessExpired={vi.fn()}
        fallbackReason="No avatar was selected for this interview."
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(
      "No avatar was selected",
    );
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    expect(screen.queryByTestId("canvas")).not.toBeInTheDocument();
  });
  it("shows a 2D fallback without mounting 3D when WebGL is unavailable", () => {
    mocks.webgl = false;
    render(
      <AvatarViewer
        access={grant("https://s3/a")}
        label="3D preview of Mia"
        onAccessExpired={vi.fn()}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(
      "3D preview isn't available on this device or browser.",
    );
    expect(screen.queryByTestId("canvas")).not.toBeInTheDocument();
  });

  it("waits for a grant, loads, then offers reset", async () => {
    const { rerender } = render(
      <AvatarViewer
        access={null}
        label="3D preview of Mia"
        onAccessExpired={vi.fn()}
      />,
    );
    expect(screen.getByRole("status")).toHaveTextContent("Loading 3D preview…");
    rerender(
      <AvatarViewer
        access={grant("https://s3/a")}
        label="3D preview of Mia"
        onAccessExpired={vi.fn()}
      />,
    );
    expect(
      await screen.findByRole("img", { name: "3D preview of Mia" }),
    ).toBeInTheDocument();
    report({ state: "ready" });
    expect(
      screen.getByRole("button", { name: "Reset view" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("re-fetches an expired grant once, then stops with an error", async () => {
    const onAccessExpired = vi.fn();
    const { rerender } = render(
      <AvatarViewer
        access={grant("https://s3/a")}
        label="Mia"
        onAccessExpired={onAccessExpired}
      />,
    );
    await screen.findByTestId("canvas");
    report({ state: "error", kind: "expired" });
    expect(onAccessExpired).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("status")).toHaveTextContent("Loading 3D preview…");

    rerender(
      <AvatarViewer
        access={grant("https://s3/b")}
        label="Mia"
        onAccessExpired={onAccessExpired}
      />,
    );
    expect(screen.getByTestId("canvas")).toHaveAttribute(
      "data-url",
      "https://s3/b",
    );
    report({ state: "error", kind: "expired" });
    expect(onAccessExpired).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("alert")).toHaveTextContent("preview link expired");

    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(onAccessExpired).toHaveBeenCalledTimes(2);
  });

  it("recovers expiry automatically again after a successful load", async () => {
    const onAccessExpired = vi.fn();
    render(
      <AvatarViewer
        access={grant("https://s3/a")}
        label="Mia"
        onAccessExpired={onAccessExpired}
      />,
    );
    await screen.findByTestId("canvas");
    report({ state: "error", kind: "expired" });
    report({ state: "ready" });
    report({ state: "error", kind: "expired" });
    expect(onAccessExpired).toHaveBeenCalledTimes(2);
  });

  it("retries a failed download by reloading the canvas", async () => {
    render(
      <AvatarViewer
        access={grant("https://s3/a")}
        label="Mia"
        onAccessExpired={vi.fn()}
      />,
    );
    await screen.findByTestId("canvas");
    report({ state: "error", kind: "network" });
    expect(screen.getByRole("alert")).toHaveTextContent(
      "couldn't download this avatar",
    );
    const mounts = mocks.mounts;
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByTestId("canvas")).toBeInTheDocument();
    expect(mocks.mounts).toBe(mounts + 1);
  });

  it("falls back to 2D when the WebGL context is lost", async () => {
    render(
      <AvatarViewer
        access={grant("https://s3/a")}
        label="Mia"
        onAccessExpired={vi.fn()}
      />,
    );
    await screen.findByTestId("canvas");
    report({ state: "fallback" });
    expect(screen.getByRole("alert")).toHaveTextContent("3D rendering stopped");
    expect(screen.queryByTestId("canvas")).not.toBeInTheDocument();
  });

  it("contains a render failure inside the viewer", async () => {
    mocks.crash = true;
    vi.spyOn(console, "error").mockImplementation(() => {});
    render(
      <div>
        <p>Page content</p>
        <AvatarViewer
          access={grant("https://s3/a")}
          label="Mia"
          onAccessExpired={vi.fn()}
        />
      </div>,
    );
    expect(
      await screen.findByText("3D preview couldn't start on this device."),
    ).toBeInTheDocument();
    expect(screen.getByText("Page content")).toBeInTheDocument();
  });
});

it("keeps preview reset by default but delegates interview view controls to the room dock", () => {
  const ready = vi.fn();
  render(
    <AvatarViewer
      access={grant("https://s3/a")}
      label="Interviewer"
      onAccessExpired={vi.fn()}
      presentation="interview"
      onReadyChange={ready}
      viewResetSignal={3}
      cameraView="close"
    />,
  );
  report({ state: "ready" });
  expect(ready).toHaveBeenLastCalledWith(true);
  expect(
    screen.queryByRole("button", { name: "Reset view" }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole("img", { name: "Interviewer" })).toBeInTheDocument();
});
