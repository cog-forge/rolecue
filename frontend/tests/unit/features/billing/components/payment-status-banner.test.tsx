import { describe, expect, it, vi, beforeEach } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { PaymentStatusBanner } from "@/features/billing/components/payment-status-banner";

const mocks = vi.hoisted(() => ({ params: new URLSearchParams() }));
vi.mock("next/navigation", () => ({ useSearchParams: () => mocks.params }));
const idle = { isPending: false, isError: false };

beforeEach(() => {
  mocks.params = new URLSearchParams("status=success&code=00&orderCode=123");
});

describe("authoritative payment feedback", () => {
  it("waits for verification despite a success redirect, then shows the backend cancellation", () => {
    const onPaymentReturn = vi.fn();
    const { rerender } = render(
      <PaymentStatusBanner
        onPaymentReturn={onPaymentReturn}
        verification={idle}
      />,
    );
    expect(screen.getByText("Verifying Payment")).toBeInTheDocument();
    expect(
      screen.queryByText("Payment Completed Successfully"),
    ).not.toBeInTheDocument();
    expect(onPaymentReturn).toHaveBeenCalledExactlyOnceWith({ orderCode: 123 });
    rerender(
      <PaymentStatusBanner
        onPaymentReturn={onPaymentReturn}
        verification={{
          ...idle,
          variables: 123,
          data: {
            id: "tx",
            amount: 100,
            currency: "COIN",
            status: "cancelled",
            payos_order_code: 123,
          },
        }}
      />,
    );
    expect(screen.getByText("Payment Cancelled")).toBeInTheDocument();
    expect(onPaymentReturn).toHaveBeenCalledTimes(1);
  });

  it.each(["pending", "failed", "success"] as const)(
    "displays verified %s independently of redirect status",
    (status) => {
      mocks.params = new URLSearchParams("cancel=true&orderCode=123");
      render(
        <PaymentStatusBanner
          onPaymentReturn={vi.fn()}
          verification={{
            ...idle,
            variables: 123,
            data: {
              id: "tx",
              amount: 100,
              currency: "COIN",
              status,
              payos_order_code: 123,
            },
          }}
        />,
      );
      expect(
        screen.getByText(
          {
            pending: "Payment Pending",
            failed: "Payment Unsuccessful",
            success: "Payment Completed Successfully",
          }[status],
        ),
      ).toBeInTheDocument();
    },
  );

  it.each([undefined, "123oops", "0", "-1", "9007199254740992"])(
    "does not verify invalid order %s",
    (code) => {
      mocks.params = new URLSearchParams(
        `status=success${code === undefined ? "" : `&orderCode=${code}`}`,
      );
      const verify = vi.fn();
      render(
        <PaymentStatusBanner onPaymentReturn={verify} verification={idle} />,
      );
      expect(screen.getByText("Unable to Verify Payment")).toBeInTheDocument();
      expect(verify).not.toHaveBeenCalled();
    },
  );

  it.each([true, false])(
    "rejects an error or mismatched response and allows retry (%s)",
    (isError) => {
      const verify = vi.fn();
      render(
        <PaymentStatusBanner
          onPaymentReturn={verify}
          verification={{
            ...idle,
            isError,
            variables: 123,
            data: {
              id: "tx",
              amount: 100,
              currency: "COIN",
              status: "success",
              payos_order_code: isError ? 123 : 456,
            },
          }}
        />,
      );
      expect(screen.getByText("Unable to Verify Payment")).toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: "Check again" }));
      expect(verify).toHaveBeenLastCalledWith({ orderCode: 123 });
      expect(
        screen.queryByText("Payment Completed Successfully"),
      ).not.toBeInTheDocument();
    },
  );

  it("does not reuse success for a different order", () => {
    render(
      <PaymentStatusBanner
        onPaymentReturn={vi.fn()}
        verification={{
          ...idle,
          variables: 456,
          data: {
            id: "tx",
            amount: 100,
            currency: "COIN",
            status: "success",
            payos_order_code: 456,
          },
        }}
      />,
    );
    expect(screen.getByText("Verifying Payment")).toBeInTheDocument();
  });
});
