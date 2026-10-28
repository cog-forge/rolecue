import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AddCoinsDialog } from "@/features/billing/components/wallet-purchase";

const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
vi.mock("@/lib/api/client", () => ({
  apiClient: { get: mocks.get, post: mocks.post },
}));

const userId = "11111111-1111-4111-8111-111111111111";
const packageId = "33333333-3333-4333-8333-333333333333";
const preflight =
  "/interviews/new/preflight?roleProfileId=22222222-2222-4222-8222-222222222222";

function renderDialog(returnPath?: string) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <AddCoinsDialog
        open
        onOpenChange={() => {}}
        userId={userId}
        role="candidate"
        returnPath={returnPath}
      />
    </QueryClientProvider>,
  );
}

async function buy() {
  fireEvent.click(
    await screen.findByRole("button", { name: "Buy Pro Pack with PayOS" }),
  );
  await waitFor(() => expect(mocks.post).toHaveBeenCalled());
  return mocks.post.mock.calls[0][1];
}

beforeEach(() => {
  vi.stubGlobal("location", { origin: "http://localhost:3000", href: "" });
  mocks.get.mockResolvedValue({
    data: {
      success: true,
      data: [
        {
          id: packageId,
          name: "Pro Pack",
          price_vnd: 100000,
          coin_amount: 250,
          is_active: true,
        },
      ],
    },
  });
  mocks.post.mockResolvedValue({
    data: {
      success: true,
      data: {
        checkout_url: "https://pay.payos.vn/checkout",
        order_code: 100300,
        amount: 100000,
        coin_amount: 250,
        transaction_id: "tx-new",
      },
    },
  });
});
afterEach(() => {
  vi.unstubAllGlobals();
});

it("returns PayOS to the screen that asked for coins, keeping its own query", async () => {
  renderDialog(preflight);
  expect(await buy()).toEqual({
    package_id: packageId,
    return_url: `http://localhost:3000${preflight}&status=success`,
    cancel_url: `http://localhost:3000${preflight}&cancel=true`,
  });
});

it.each(["//evil.example/steal", "/\\evil.example", "https://evil.example/"])(
  "falls back to Billing for a return path that would leave the site (%s)",
  async (returnPath) => {
    renderDialog(returnPath);
    expect(await buy()).toMatchObject({
      return_url: "http://localhost:3000/billing?status=success",
    });
  },
);
