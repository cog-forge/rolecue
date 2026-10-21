import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { TransactionHistory } from "@/features/billing/components/transaction-history";
import type { TransactionPage } from "@/features/billing/schemas/wallet-schema";

it("scopes filters and counts to the current page, preserving access to matching later pages", () => {
  const props = {
    currentUserId: "user",
    isLoading: false,
    isFetching: false,
    error: null,
    page: 1,
    onPageChange: vi.fn(),
    onRefresh: vi.fn(),
  };
  const firstPage: TransactionPage = {
    items: Array.from({ length: 20 }, (_, i) => ({
      id: `deposit-${i}`,
      to: "user",
      amount: 100,
      currency: "COIN",
      status: "success",
      payos_order_code: i + 1,
    })),
    total: 21,
    page: 1,
    page_size: 20,
  };
  const { rerender } = render(
    <TransactionHistory {...props} data={firstPage} />,
  );
  fireEvent.mouseDown(screen.getByRole("tab", { name: "Coin Spending" }), {
    button: 0,
    ctrlKey: false,
  });
  expect(screen.getByRole("tab", { name: "Coin Spending" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  expect(screen.getByRole("tabpanel")).toHaveAccessibleName("Coin Spending");
  expect(
    screen.getByText(
      "No spend transactions on this page. Other pages may contain matching transactions.",
    ),
  ).toBeInTheDocument();
  expect(
    screen.getByText("Showing 0 of 20 transactions on this page."),
  ).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Next page" }));
  expect(props.onPageChange).toHaveBeenCalledWith(2);
  rerender(
    <TransactionHistory
      {...props}
      page={2}
      data={{
        ...firstPage,
        page: 2,
        items: [
          {
            id: "spend",
            from: "user",
            amount: 20,
            currency: "COIN",
            status: "success",
            description: "Practice",
          },
        ],
      }}
    />,
  );
  expect(screen.getByText("Practice")).toBeInTheDocument();
  expect(
    screen.getByText("Showing 1 of 1 transactions on this page."),
  ).toBeInTheDocument();
});

describe("unavailable transaction dates", () => {
  it.each([undefined, "0001-01-01T00:00:00Z", "invalid"])(
    "renders a placeholder for %s",
    (created_at) => {
      render(
        <TransactionHistory
          currentUserId="user"
          isLoading={false}
          isFetching={false}
          error={null}
          page={1}
          onPageChange={vi.fn()}
          onRefresh={vi.fn()}
          data={{
            items: [
              {
                id: "tx",
                amount: 1,
                currency: "COIN",
                status: "pending",
                created_at,
              },
            ],
            total: 1,
            page: 1,
            page_size: 20,
          }}
        />,
      );
      expect(screen.getByText("—")).toBeInTheDocument();
      expect(screen.getByText("Pending")).not.toHaveClass("animate-pulse");
    },
  );
});
