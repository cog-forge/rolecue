import { useEffect } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ModelAccess } from "@/components/avatar-viewer/avatar-viewer";
import { InterviewerPreview } from "@/features/recruiter/job-postings/components/interviewer-preview";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  viewer: vi.fn(),
  mounts: 0,
  unmounts: 0,
}));
vi.mock("@/lib/api/client", () => ({ apiClient: { get: mocks.get } }));
vi.mock("@/components/avatar-viewer/avatar-viewer", () => ({
  AvatarViewer: ({
    access,
    label,
    onAccessExpired,
  }: {
    access: ModelAccess | null;
    label: string;
    onAccessExpired: () => void;
  }) => {
    useEffect(() => {
      mocks.mounts += 1;
      return () => {
        mocks.unmounts += 1;
      };
    }, []);
    mocks.viewer(access);
    return (
      <div>
        <div role="img" aria-label={label} data-url={access?.url ?? ""} />
        <button type="button" onClick={onAccessExpired}>
          Refresh expired preview
        </button>
      </div>
    );
  },
}));

const userId = "11111111-1111-4111-8111-111111111111";
const linhId = "44444444-4444-4444-8444-444444444444";
const miaId = "55555555-5555-4555-8555-555555555555";
const avatar = (
  id = linhId,
  name = "Linh",
  url = "https://assets.example/linh.vrm",
) => ({
  data: {
    success: true,
    data: {
      id,
      name,
      personality: "Calm and precise",
      created_at: "2026-10-01T00:00:00Z",
      updated_at: "2026-10-01T00:00:00Z",
      model_access: {
        url,
        method: "GET",
        headers: { "x-preview-access": "owner-grant" },
        expires_at: "2026-10-29T12:05:00Z",
      },
    },
  },
});

function mount(selectedId = linhId) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  const view = (avatarId: string) => (
    <QueryClientProvider client={client}>
      <InterviewerPreview userId={userId} avatarId={avatarId} />
    </QueryClientProvider>
  );
  const result = render(view(selectedId));
  return {
    rerender: (avatarId = selectedId) => result.rerender(view(avatarId)),
  };
}

beforeEach(() => {
  mocks.get.mockReset();
  mocks.viewer.mockClear();
  mocks.mounts = 0;
  mocks.unmounts = 0;
});

describe("InterviewerPreview", () => {
  it("loads the selected avatar's private grant and keeps its access stable on rerender", async () => {
    mocks.get.mockResolvedValue(avatar());
    const view = mount();

    expect(
      await screen.findByRole("img", { name: "3D preview of Linh" }),
    ).toHaveAttribute("data-url", "https://assets.example/linh.vrm");
    expect(mocks.get).toHaveBeenCalledWith(
      `/inventory/avatars/${linhId}`,
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
    const access = mocks.viewer.mock.calls.at(-1)?.[0] as ModelAccess;
    expect(access).toEqual({
      url: "https://assets.example/linh.vrm",
      method: "GET",
      headers: { "x-preview-access": "owner-grant" },
      expiresAt: "2026-10-29T12:05:00Z",
    });

    view.rerender();
    expect(mocks.viewer.mock.calls.at(-1)?.[0]).toBe(access);
    expect(mocks.get).toHaveBeenCalledTimes(1);
  });

  it("drops the previous model grant while the newly selected interviewer loads", async () => {
    let loadMia: ((response: ReturnType<typeof avatar>) => void) | undefined;
    mocks.get.mockResolvedValueOnce(avatar()).mockImplementationOnce(
      () =>
        new Promise<ReturnType<typeof avatar>>((resolve) => {
          loadMia = resolve;
        }),
    );
    const view = mount();
    await screen.findByRole("img", { name: "3D preview of Linh" });

    view.rerender(miaId);
    expect(
      await screen.findByRole("img", {
        name: "3D preview of selected interviewer",
      }),
    ).toHaveAttribute("data-url", "");
    expect(
      screen.queryByRole("img", { name: "3D preview of Linh" }),
    ).not.toBeInTheDocument();
    await waitFor(() => expect(loadMia).toBeDefined());
    await act(async () =>
      loadMia?.(avatar(miaId, "Mia", "https://assets.example/mia.vrm")),
    );

    expect(
      await screen.findByRole("img", { name: "3D preview of Mia" }),
    ).toHaveAttribute("data-url", "https://assets.example/mia.vrm");
  });

  it("recovers a failed preview without changing the selected interviewer", async () => {
    mocks.get
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValue(avatar());
    mount();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Your selection is still saved in the form.",
    );
    fireEvent.click(screen.getByRole("button", { name: "Retry preview" }));

    expect(
      await screen.findByRole("img", { name: "3D preview of Linh" }),
    ).toHaveAttribute("data-url", "https://assets.example/linh.vrm");
    expect(mocks.get).toHaveBeenCalledTimes(2);
    expect(mocks.get.mock.calls.map(([path]) => path)).toEqual([
      `/inventory/avatars/${linhId}`,
      `/inventory/avatars/${linhId}`,
    ]);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("refreshes an expired grant for the same selected avatar", async () => {
    mocks.get
      .mockResolvedValueOnce(avatar())
      .mockResolvedValue(
        avatar(linhId, "Linh", "https://assets.example/linh-renewed.vrm"),
      );
    mount();
    await screen.findByRole("img", { name: "3D preview of Linh" });

    fireEvent.click(
      screen.getByRole("button", { name: "Refresh expired preview" }),
    );

    await waitFor(() =>
      expect(
        screen.getByRole("img", { name: "3D preview of Linh" }),
      ).toHaveAttribute("data-url", "https://assets.example/linh-renewed.vrm"),
    );
    expect(mocks.get).toHaveBeenCalledTimes(2);
  });

  it("keeps the viewer mounted when its grant refreshes so expiry retry limits survive", async () => {
    mocks.get
      .mockResolvedValueOnce(avatar())
      .mockResolvedValue(
        avatar(linhId, "Linh", "https://assets.example/linh-renewed.vrm"),
      );
    mount();
    await screen.findByRole("img", { name: "3D preview of Linh" });
    const mountsBeforeRefresh = mocks.mounts;
    const unmountsBeforeRefresh = mocks.unmounts;

    fireEvent.click(
      screen.getByRole("button", { name: "Refresh expired preview" }),
    );
    await waitFor(() =>
      expect(
        screen.getByRole("img", { name: "3D preview of Linh" }),
      ).toHaveAttribute("data-url", "https://assets.example/linh-renewed.vrm"),
    );

    expect(mocks.mounts).toBe(mountsBeforeRefresh);
    expect(mocks.unmounts).toBe(unmountsBeforeRefresh);
  });
});
