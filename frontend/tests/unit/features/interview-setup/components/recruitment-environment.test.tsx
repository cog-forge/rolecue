import { beforeEach, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { UseQueryResult } from "@tanstack/react-query";
import { RecruitmentEnvironment } from "@/features/interview-setup/components/recruitment-environment";
import type { SavedSetup } from "@/features/interview-setup/api/saved-setup";
import {
  applicationId,
  envelope,
  recruitmentView,
} from "../saved-setup-fixtures";

const mocks = vi.hoisted(() => ({
  put: vi.fn(),
  user: { id: "11111111-1111-4111-8111-111111111111" },
}));
vi.mock("@/lib/api/client", () => ({ apiClient: { put: mocks.put } }));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));

const putPath = `/interview-setups/recruitment/${applicationId}`;

const asQuery = (data: SavedSetup) =>
  ({ data, isError: false }) as UseQueryResult<SavedSetup>;

function renderSection(data: SavedSetup) {
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  });
  return renderWithIntl(
    <QueryClientProvider client={client}>
      <RecruitmentEnvironment
        applicationId={applicationId}
        setup={asQuery(data)}
      />
    </QueryClientProvider>,
  );
}

beforeEach(() => mocks.put.mockReset());

it("shows the saved room", () => {
  renderSection(recruitmentView({ saved: true, environment: "balcony" }));
  expect(screen.getByRole("radio", { name: "Balcony" })).toBeChecked();
});

it("explains the missing room", () => {
  renderSection(recruitmentView());
  expect(screen.getByText("Choose an interview room.")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Choose a room" })).toHaveAttribute(
    "href",
    "#setup-environment",
  );
});

it("sends the environment only, never the locked avatar or voice", async () => {
  mocks.put.mockResolvedValue(
    envelope(
      recruitmentView({
        saved: true,
        environment: "cafe-3d",
        ready: true,
        blockers: [],
      }),
    ),
  );
  renderSection(recruitmentView());
  fireEvent.click(screen.getByRole("radio", { name: "Café (3D room)" }));
  await waitFor(() => expect(mocks.put).toHaveBeenCalledTimes(1));
  const [url, body] = mocks.put.mock.calls[0] as [
    string,
    Record<string, unknown>,
  ];
  expect(url).toBe(putPath);
  expect(body).toEqual({ environment: expect.any(String) });
  expect(body).not.toHaveProperty("avatar_id");
  expect(body).not.toHaveProperty("voice_id");
  expect(await screen.findByText("Saved")).toBeInTheDocument();
});

it("shows a failed save with a retry", async () => {
  mocks.put.mockRejectedValueOnce(new Error("offline"));
  mocks.put.mockResolvedValueOnce(
    envelope(recruitmentView({ saved: true, environment: "balcony" })),
  );
  renderSection(recruitmentView());
  fireEvent.click(screen.getByRole("radio", { name: "Balcony" }));
  expect(
    await screen.findByText("We couldn't save your choices."),
  ).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Try again" }));
  await waitFor(() => expect(mocks.put).toHaveBeenCalledTimes(2));
  expect(mocks.put.mock.calls[1]).toEqual(mocks.put.mock.calls[0]);
});
