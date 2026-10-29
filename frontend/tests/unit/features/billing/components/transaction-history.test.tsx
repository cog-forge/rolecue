import { describe, expect, it, vi } from "vitest";
import { fireEvent, screen } from "@testing-library/react";
import { renderWithIntl } from "@tests/helpers/intl";
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
  const { rerender } = renderWithIntl(
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
      renderWithIntl(
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

it("shows when each transaction happened in the viewer's local time", () => {
  const createdAt = "2026-10-27T06:16:40Z";
  const expected = new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(createdAt));

  renderWithIntl(
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
            from: "user",
            amount: 20,
            currency: "COIN",
            status: "success",
            description: "Practice",
            created_at: createdAt,
          },
        ],
        total: 1,
        page: 1,
        page_size: 20,
      }}
    />,
  );

  const time = screen.getByText(expected);
  expect(time.tagName).toBe("TIME");
  expect(time).toHaveAttribute("datetime", createdAt);
});

describe("what each row says", () => {
  const base = {
    currentUserId: "user",
    isLoading: false,
    isFetching: false,
    error: null,
    page: 1,
    onPageChange: vi.fn(),
    onRefresh: vi.fn(),
  };

  it("names ledger rows by what happened, without internal IDs", () => {
    renderWithIntl(
      <TransactionHistory
        {...base}
        data={{
          items: [
            {
              id: "spend",
              from: "user",
              to: "system",
              amount: 5,
              currency: "COIN",
              status: "success",
              description:
                "Practice interview 5c1f2e8a-0000-4000-8000-000000000001",
            },
          ],
          total: 1,
          page: 1,
          page_size: 20,
        }}
      />,
    );
    expect(screen.getByText("Practice interview")).toBeInTheDocument();
    expect(screen.queryByText(/5c1f2e8a/)).not.toBeInTheDocument();
  });

  it("does not show coins for a top-up that was never paid", () => {
    renderWithIntl(
      <TransactionHistory
        {...base}
        data={{
          items: [
            {
              id: "failed",
              to: "user",
              amount: 200,
              currency: "VND",
              status: "failed",
              description: "Deposit: Plus (200000 VND)",
              payos_order_code: 191000004,
            },
          ],
          total: 1,
          page: 1,
          page_size: 20,
        }}
      />,
    );
    expect(screen.getByText("Plus pack")).toBeInTheDocument();
    expect(
      screen.getByRole("cell", { name: "No coins added" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("+200")).not.toBeInTheDocument();
  });
});
