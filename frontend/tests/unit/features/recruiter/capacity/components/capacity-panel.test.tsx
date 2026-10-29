import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import { CapacityPanel } from "@/features/recruiter/capacity/components/capacity-panel";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  user: {
    id: "11111111-1111-4111-8111-111111111111",
    full_name: "Rec",
    email: "rec@example.com",
    role: "recruiter" as const,
    email_verified: true,
    is_locked: false,
    image: null,
  },
}));
vi.mock("@/lib/api/client", () => ({
  apiClient: { get: mocks.get, post: mocks.post },
}));
vi.mock("@/features/auth/components/session-gate", () => ({
  useSessionUser: () => mocks.user,
}));

const postingId = "33333333-3333-4333-8333-333333333333";
const capacityPath = `/job-postings/${postingId}/interview-capacity`;
const applicationsPath = `/job-postings/${postingId}/applications`;
const closeIntakePath = `/job-postings/${postingId}/close-intake`;

const envelope = <T,>(data: T) => ({ data: { success: true, data } });
function failure(status: number, code: string, message: string) {
  return new AxiosError("failure", "ERR_BAD_RESPONSE", undefined, undefined, {
    status,
    statusText: "Error",
    data: { success: false, error: { code, message } },
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  });
}

type Capacity = {
  interview_slot: number | null;
  funded: boolean;
  price_per_slot: number;
  cv_approvals: number;
  consumed: number;
  available_to_approve: number;
  refundable: number;
  posting_status: "pending" | "open" | "intake_closed" | "closed" | "rejected";
  refunded: number | null;
};
const unfunded: Capacity = {
  interview_slot: null,
  funded: false,
  price_per_slot: 1,
  cv_approvals: 0,
  consumed: 0,
  available_to_approve: 0,
  refundable: 0,
  posting_status: "open",
  refunded: null,
};
const funded: Capacity = {
  interview_slot: 5,
  funded: true,
  price_per_slot: 1,
  cv_approvals: 2,
  consumed: 1,
  available_to_approve: 3,
  refundable: 4,
  posting_status: "open",
  refunded: null,
};

// Serves the capacity state (which can change between reads) and the submitted-CV total.
let capacity: Capacity | Error;
function serve(state: Capacity | Error, submitted = 0) {
  capacity = state;
  mocks.get.mockImplementation((path: string) => {
    if (path === capacityPath)
      return capacity instanceof Error
        ? Promise.reject(capacity)
        : Promise.resolve(envelope(capacity));
    if (path === applicationsPath)
      return Promise.resolve(
        envelope({
          items: [],
          pagination: { limit: 20, offset: 0, total: submitted },
        }),
      );
    return Promise.reject(new Error(`unexpected GET ${path}`));
  });
}

function mount() {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity },
      mutations: { retry: false },
    },
  });
  renderWithIntl(
    <QueryClientProvider client={client}>
      <CapacityPanel postingId={postingId} />
    </QueryClientProvider>,
  );
  return client;
}

function fundingForm() {
  return screen.queryByRole("form", { name: "Fund interview slots" });
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.get.mockReset();
  mocks.post.mockReset();
});

describe("CapacityPanel funding", () => {
  it("funds an open, unfunded posting and then shows the funded state", async () => {
    serve(unfunded);
    mocks.post.mockImplementation(() => {
      serve(funded, 0);
      return Promise.resolve(envelope(funded));
    });
    mount();

    expect(
      await screen.findByText(
        "Approved by an administrator. Fund interview slots to open intake: candidates can't see or apply to this posting until you do.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Candidates can apply once you fund interview slots."),
    ).toBeInTheDocument();
    const input = screen.getByLabelText("Interview slots to fund");
    fireEvent.change(input, { target: { value: "5" } });
    expect(
      screen.getByText("Total: 5 coins from your wallet."),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Fund interview slots" }),
    );

    await waitFor(() =>
      expect(mocks.post).toHaveBeenCalledWith(capacityPath, { slots: 5 }),
    );
    expect(await screen.findByText("Funded slots")).toBeInTheDocument();
    expect(fundingForm()).not.toBeInTheDocument();
  });

  it("refreshes the wallet balance after funding", async () => {
    serve(unfunded);
    mocks.post.mockImplementation(() => {
      serve(funded, 0);
      return Promise.resolve(envelope(funded));
    });
    const client = mount();
    const invalidate = vi.spyOn(client, "invalidateQueries");

    fireEvent.change(await screen.findByLabelText("Interview slots to fund"), {
      target: { value: "5" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Fund interview slots" }),
    );

    await waitFor(() =>
      expect(invalidate).toHaveBeenCalledWith({
        queryKey: ["wallet", mocks.user.id],
      }),
    );
  });

  it("keeps the form and explains a low balance without changing anything", async () => {
    serve(unfunded);
    mocks.post.mockRejectedValue(
      failure(402, "INSUFFICIENT_BALANCE", "wallet is short"),
    );
    mount();

    fireEvent.change(await screen.findByLabelText("Interview slots to fund"), {
      target: { value: "500" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Fund interview slots" }),
    );

    expect(
      await screen.findByText(
        "Your wallet does not have enough coins for this many slots. Nothing was charged.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Interview slots to fund")).toHaveValue(500);
    expect(
      screen.getByRole("button", { name: "Fund interview slots" }),
    ).toBeEnabled();
  });

  it("rejects a slot count outside 1 to 10000 before calling the API", async () => {
    serve(unfunded);
    mount();
    const input = await screen.findByLabelText("Interview slots to fund");

    fireEvent.change(input, { target: { value: "0" } });
    fireEvent.click(
      screen.getByRole("button", { name: "Fund interview slots" }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Fund at least 1 slot.",
    );

    fireEvent.change(input, { target: { value: "10001" } });
    fireEvent.click(
      screen.getByRole("button", { name: "Fund interview slots" }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Fund at most 10,000 slots.",
    );

    fireEvent.change(input, { target: { value: "1.5" } });
    fireEvent.click(
      screen.getByRole("button", { name: "Fund interview slots" }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Enter a whole number of slots.",
    );
    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("maps an already-funded posting to plain copy and reloads the capacity", async () => {
    serve(unfunded);
    mocks.post.mockRejectedValue(
      failure(409, "INTERVIEW_CAPACITY_ALREADY_FUNDED", "funded already"),
    );
    mount();

    fireEvent.change(await screen.findByLabelText("Interview slots to fund"), {
      target: { value: "3" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Fund interview slots" }),
    );

    expect(
      await screen.findByText(
        "Interview slots are already funded for this posting. The current count is shown below.",
      ),
    ).toBeInTheDocument();
  });

  it("does not offer funding once intake is closed", async () => {
    serve({ ...unfunded, posting_status: "intake_closed" });
    mount();

    expect(
      await screen.findByText(
        "Intake is closed, so interview slots can no longer be funded.",
      ),
    ).toBeInTheDocument();
    expect(fundingForm()).not.toBeInTheDocument();
  });

  it("tells a pending posting that funding comes after approval and opens intake", async () => {
    serve({ ...unfunded, posting_status: "pending" });
    mount();

    expect(
      await screen.findByText(
        "Interview slots can be funded once an administrator approves the posting. Funding them opens intake to candidates.",
      ),
    ).toBeInTheDocument();
    expect(fundingForm()).not.toBeInTheDocument();
  });

  it("shows slot capacity separately from the submitted-CV count", async () => {
    serve(funded, 7);
    mount();

    const slots = await screen.findByRole("region", {
      name: "Interview slots",
    });
    expect(
      within(slots).getByText("Still open for approval").nextElementSibling,
    ).toHaveTextContent("3");
    expect(
      within(slots).getByText("Interviews started").nextElementSibling,
    ).toHaveTextContent("1");

    const submitted = screen.getByRole("region", { name: "CVs submitted" });
    expect(await within(submitted).findByText("7")).toBeInTheDocument();
    expect(
      within(submitted).getByText(/do not use interview slots/i),
    ).toBeInTheDocument();
    expect(fundingForm()).not.toBeInTheDocument();
  });

  it("does not count submissions for a pending posting", async () => {
    serve({ ...unfunded, posting_status: "pending" });
    mount();

    expect(
      await screen.findByText("Candidates can apply once intake opens."),
    ).toBeInTheDocument();
    expect(mocks.get).not.toHaveBeenCalledWith(
      applicationsPath,
      expect.anything(),
    );
  });
});

describe("CapacityPanel close intake", () => {
  it("confirms before closing, closes once and then removes the control", async () => {
    serve(funded, 4);
    mocks.post.mockImplementation((path: string) => {
      if (path === closeIntakePath) {
        serve({ ...funded, posting_status: "intake_closed" }, 4);
        return Promise.resolve({ data: { success: true, data: {} } });
      }
      return Promise.reject(new Error(`unexpected POST ${path}`));
    });
    mount();

    fireEvent.click(
      await screen.findByRole("button", { name: "Close intake" }),
    );
    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByText(/Applications still pending will be rejected/),
    ).toBeInTheDocument();

    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    expect(mocks.post).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Close intake" }));
    const confirm = within(await screen.findByRole("dialog")).getByRole(
      "button",
      { name: "Close intake" },
    );
    fireEvent.click(confirm);

    await waitFor(() =>
      expect(
        screen.queryByRole("button", { name: "Close intake" }),
      ).not.toBeInTheDocument(),
    );
    expect(mocks.post).toHaveBeenCalledTimes(1);
    expect(mocks.post).toHaveBeenCalledWith(closeIntakePath);
  });

  it("offers no reopen or end-recruitment control", async () => {
    serve(funded);
    mount();

    await screen.findByRole("button", { name: "Close intake" });
    expect(
      screen.queryByRole("button", { name: /reopen|end recruitment/i }),
    ).not.toBeInTheDocument();
  });
});

describe("CapacityPanel closed posting", () => {
  it("shows the refund read-only and no actions", async () => {
    serve({
      ...funded,
      posting_status: "closed",
      consumed: 2,
      available_to_approve: 0,
      refundable: 3,
      refunded: 3,
    });
    mount();

    expect(
      await screen.findByText(
        "3 coins were returned to your wallet when this posting closed.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(fundingForm()).not.toBeInTheDocument();
  });
});

describe("CapacityPanel closed without funding", () => {
  it("says nothing was funded instead of showing a refund", async () => {
    serve({
      ...unfunded,
      posting_status: "closed",
      refunded: 0,
    });
    mount();

    expect(
      await screen.findByText(
        "This posting closed without funded interview slots, so there was nothing to return to your wallet.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Interview slots refunded"),
    ).not.toBeInTheDocument();
  });
});

describe("CapacityPanel errors", () => {
  it("shows the not-found state when the posting is not the Recruiter's", async () => {
    serve(failure(404, "JOB_POSTING_NOT_FOUND", "missing"));
    mount();

    expect(
      await screen.findByRole("heading", {
        name: "We couldn't find this job posting",
      }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Try again" }),
    ).not.toBeInTheDocument();
  });

  it("lets the Recruiter try again after a server error", async () => {
    serve(failure(500, "INTERNAL", "boom"));
    mount();

    fireEvent.click(await screen.findByRole("button", { name: "Try again" }));
    await waitFor(() =>
      expect(
        mocks.get.mock.calls.filter(([path]) => path === capacityPath),
      ).toHaveLength(2),
    );
  });
});
