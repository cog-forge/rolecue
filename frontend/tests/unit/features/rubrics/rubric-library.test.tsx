import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithIntl } from "../../../helpers/intl";
import { TooltipProvider } from "@/components/ui/tooltip";
import { RubricLibrary } from "@/features/rubrics/rubric-library";
import type { Rubric } from "@/features/rubrics/rubric-schema";
const mocks = vi.hoisted(() => ({
  list: vi.fn(),
  get: vi.fn(),
  metrics: vi.fn(),
  write: vi.fn(),
  copy: vi.fn(),
  remove: vi.fn(),
}));
vi.mock("@/features/rubrics/rubric-api", () => ({
  rubricKeys: { all: (id: string) => ["rubrics", id] },
  listRubrics: mocks.list,
  getRubric: mocks.get,
  getRubricMetrics: mocks.metrics,
  writeRubric: mocks.write,
  copyRubric: mocks.copy,
  deleteRubric: mocks.remove,
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => ({ id: "review-user" }),
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));
const metric = "22222222-2222-4222-8222-222222222222";
const rubric: Rubric = {
  id: "11111111-1111-4111-8111-111111111111",
  name: "Mechanism rubric",
  description: "",
  question_type: "concept_explanation",
  scope: "system",
  is_active: true,
  updated_at: "2026-11-11T06:00:00Z",
  created_at: "2026-11-11T06:00:00Z",
  criteria_count: 1,
  max_points: 3,
  is_own: false,
  can_manage: true,
  locked: false,
  reference_count: 0,
  criteria: [
    {
      id: metric,
      metric_id: metric,
      name: "Accuracy",
      max_points: 3,
      scoring_guide: "0 absent; 3 complete",
    },
  ],
};
function mount() {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity },
      mutations: { retry: false },
    },
  });
  renderWithIntl(
    <QueryClientProvider client={client}>
      <TooltipProvider>
        <RubricLibrary role="admin" />
      </TooltipProvider>
    </QueryClientProvider>,
  );
  return client;
}
async function edit() {
  await screen.findByRole("button", { name: "Edit" });
  fireEvent.click(screen.getByRole("button", { name: "Edit" }));
  await screen.findByLabelText("Rubric name");
}
beforeEach(() => {
  vi.resetAllMocks();
  mocks.list.mockResolvedValue({ items: [rubric], pagination: { total: 1 } });
  mocks.get.mockResolvedValue(rubric);
  mocks.metrics.mockResolvedValue([{ id: metric, name: "Accuracy" }]);
  mocks.write.mockResolvedValue(rubric);
});
describe("rubric editor draft safety", () => {
  it("keeps the loaded draft token when background data advances", async () => {
    const client = mount();
    await edit();
    fireEvent.change(screen.getByLabelText("Rubric name"), {
      target: { value: "Local draft" },
    });
    mocks.get.mockResolvedValue({
      ...rubric,
      name: "Remote edit",
      updated_at: "2026-11-11T07:00:00Z",
    });
    await act(async () => {
      await client.refetchQueries({
        queryKey: ["rubrics", "review-user", "detail", rubric.id],
      });
    });
    fireEvent.click(screen.getByRole("button", { name: "Save rubric" }));
    await waitFor(() =>
      expect(mocks.write).toHaveBeenCalledWith({
        id: rubric.id,
        body: expect.objectContaining({
          name: "Local draft",
          updated_at: rubric.updated_at,
        }),
      }),
    );
  });
  it("preserves dirty fields and guards through failed background detail and metric refreshes", async () => {
    const client = mount();
    await edit();
    fireEvent.change(screen.getByLabelText("Rubric name"), {
      target: { value: "Retained draft" },
    });
    mocks.get.mockRejectedValue(new Error("offline"));
    mocks.metrics.mockRejectedValue(new Error("offline"));
    await act(async () => {
      await client.refetchQueries({ queryKey: ["rubrics", "review-user"] });
    });
    expect(screen.getByLabelText("Rubric name")).toHaveValue("Retained draft");
    expect(screen.getByText("Unsaved changes")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Back to library" }));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });
  it("retries the metric catalog during creation without requesting a new-rubric detail", async () => {
    mocks.metrics
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValue([{ id: metric, name: "Accuracy" }]);
    mount();
    await screen.findByRole("button", { name: "Edit" });
    fireEvent.click(screen.getByRole("button", { name: "Create rubric" }));
    await screen.findByRole("button", { name: "Try again" });
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    await screen.findByLabelText("Rubric name");
    expect(mocks.get).not.toHaveBeenCalled();
    expect(mocks.metrics).toHaveBeenCalledTimes(2);
  });
  it("prevents competing editor navigation while a copy is pending", async () => {
    mocks.copy.mockReturnValue(new Promise(() => {}));
    mount();
    await screen.findByRole("button", { name: "Copy" });
    fireEvent.click(screen.getByRole("button", { name: "Copy" }));
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Create rubric" }),
      ).toBeDisabled(),
    );
    expect(screen.getByRole("button", { name: "Edit" })).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Mechanism rubric" }),
    ).toBeDisabled();
  });
  it("blocks editing and overlapping recovery actions during an explicit reload", async () => {
    const stale = Object.assign(new Error("stale"), {
      isAxiosError: true,
      response: { status: 409, data: { error: { code: "RUBRIC_STALE" } } },
    });
    mocks.write.mockRejectedValue(stale);
    mount();
    await edit();
    fireEvent.change(screen.getByLabelText("Rubric name"), {
      target: { value: "Draft" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save rubric" }));
    await screen.findByRole("button", { name: "Reload latest" });
    mocks.get.mockReturnValue(new Promise(() => {}));
    fireEvent.click(screen.getByRole("button", { name: "Reload latest" }));
    fireEvent.click(screen.getByRole("button", { name: "Confirm" }));
    await waitFor(() =>
      expect(screen.getByLabelText("Rubric name")).toBeDisabled(),
    );
    expect(
      screen.getByRole("button", { name: "Reload latest" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Keep edits with latest revision" }),
    ).toBeDisabled();
  });
});
