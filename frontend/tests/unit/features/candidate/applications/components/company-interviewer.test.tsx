import { afterEach, expect, it, vi } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderWithIntl } from "@tests/helpers/intl";
import { CompanyInterviewer } from "@/features/candidate/applications/components/company-interviewer";

const mocks = vi.hoisted(() => ({ get: vi.fn(), render: vi.fn() }));
vi.mock("@/lib/api/client", () => ({ apiClient: { get: mocks.get } }));
vi.mock("@/components/avatar-viewer/render-portrait", () => ({
  renderAvatarPortrait: mocks.render,
}));
const grant = {
  url: "https://assets.example.test/company.vrm",
  method: "GET",
  headers: {},
  expires_at: "2099-01-01T00:00:00Z",
};
function mount(locale: "en" | "vi" = "en") {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  mocks.get.mockResolvedValue({
    data: {
      success: true,
      data: {
        name: "Company model",
        voice_name: "Company voice",
        model_access: grant,
      },
    },
  });
  return renderWithIntl(
    <QueryClientProvider client={client}>
      <CompanyInterviewer userId="candidate" applicationId="application" />
    </QueryClientProvider>,
    { locale },
  );
}
afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});
it("renders only the granted company model and revokes its portrait on unmount", async () => {
  mocks.render.mockResolvedValue("blob:company-portrait");
  const revoke = vi.fn();
  vi.stubGlobal(
    "URL",
    class extends URL {
      static revokeObjectURL = revoke;
    },
  );
  const view = mount();
  expect(await screen.findByText("Voice: Company voice")).toBeVisible();
  await waitFor(() =>
    expect(view.container.querySelector("img")).toHaveAttribute(
      "src",
      "blob:company-portrait",
    ),
  );
  expect(mocks.get).toHaveBeenCalledWith(
    "/applications/application/interviewer",
    expect.objectContaining({ signal: expect.any(AbortSignal) }),
  );
  view.unmount();
  expect(revoke).toHaveBeenCalledWith("blob:company-portrait");
});
it("keeps a Vietnamese 2D fallback when rendering the model fails", async () => {
  mocks.render.mockRejectedValue(new Error("WebGL unavailable"));
  mount("vi");
  expect(await screen.findByText("Giọng nói: Company voice")).toBeVisible();
  expect(screen.getByText(/Vẫn có người phỏng vấn 2D/)).toBeVisible();
});
