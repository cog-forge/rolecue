import { describe, expect, it } from "vitest";
import { fireEvent, render } from "@testing-library/react";
import { UserAvatar } from "@/components/account/user-avatar";

describe("account avatar", () => {
  it("uses Peek when a remote image fails, and tries a changed source", () => {
    const { container, rerender } = render(
      <UserAvatar name="Nguyễn Nam" image="https://example.com/a.png" />,
    );
    const image = container.querySelector("img");
    expect(image).not.toBeNull();
    expect(container.querySelector("svg")).toBeNull();
    fireEvent.error(image!);
    expect(container.querySelector("svg")).toBeInTheDocument();
    expect(container.querySelector("img")).toBeNull();
    rerender(
      <UserAvatar name="Nguyễn Nam" image="https://example.com/b.png" />,
    );
    expect(container.querySelector("img")).toHaveAttribute(
      "src",
      "https://example.com/b.png",
    );
  });
  it("does not load unsafe legacy sources or URLs with credentials", () => {
    const { container, rerender } = render(
      <UserAvatar name="Alex Doe" image="javascript:alert(1)" />,
    );
    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("svg")).toBeInTheDocument();
    rerender(
      <UserAvatar name="" image="https://user:password@example.com/a" />,
    );
    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("svg")).toBeInTheDocument();
  });
  it("keeps the generated face when the same user changes their display name", () => {
    const { container, rerender } = render(
      <UserAvatar id="candidate-1" name="Alex Doe" />,
    );
    const face = container.querySelector("svg")!.innerHTML;
    rerender(<UserAvatar id="candidate-1" name="Alex Nguyen" />);
    expect(container.querySelector("svg")!.innerHTML).toBe(face);
    rerender(<UserAvatar id="recruiter-2" name="Alex Nguyen" />);
    expect(container.querySelector("svg")!.innerHTML).not.toBe(face);
    expect(container.querySelector("svg")).toHaveAttribute(
      "aria-hidden",
      "true",
    );
  });
});
