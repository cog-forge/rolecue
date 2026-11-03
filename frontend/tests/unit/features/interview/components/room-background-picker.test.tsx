import { fireEvent, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Toolbar } from "radix-ui";
import { TooltipProvider } from "@/components/ui/tooltip";
import type { EnvironmentStatus } from "@/components/avatar-viewer/environment/environment-state";
import { RoomBackgroundPicker } from "@/features/interview/components/room-background-picker";
import type { RoomBackgroundId } from "@/features/interview/components/room-backgrounds";
import { renderWithIntl } from "@tests/helpers/intl";

function picker(
  sceneStatus: EnvironmentStatus,
  {
    value = "cafe-3d",
    onChange = vi.fn(),
    locale = "en",
  }: {
    value?: RoomBackgroundId;
    onChange?: (id: RoomBackgroundId) => void;
    locale?: "en" | "vi";
  } = {},
) {
  return renderWithIntl(
    <TooltipProvider>
      <Toolbar.Root>
        <RoomBackgroundPicker
          value={value}
          onChange={onChange}
          sceneStatus={sceneStatus}
        />
      </Toolbar.Root>
    </TooltipProvider>,
    { locale },
  );
}

const open = (name = "Choose background") =>
  fireEvent.click(screen.getByRole("button", { name }));

describe("RoomBackgroundPicker", () => {
  it("lists the 3D rooms with thumbnails ahead of the lightweight photos", () => {
    picker({ state: "ready" });
    open();
    const panel = screen.getByRole("dialog", { name: "Room backgrounds" });
    const radios = within(panel).getAllByRole("radio");
    expect(
      radios.map(
        (radio) => radio.getAttribute("aria-label") ?? radio.textContent,
      ),
    ).toEqual([
      "Café (3D room)",
      "Office (3D room)",
      "Meeting room (3D room)",
      "Office",
      "Balcony",
      "Café",
    ]);
    expect(radios[0]).toHaveAttribute("aria-checked", "true");
    expect(
      within(radios[1]).getByRole("presentation", { hidden: true }),
    ).toHaveAttribute("src", expect.stringContaining("office%2Fthumb.webp"));
    expect(within(panel).getByText("3D rooms")).toBeVisible();
    expect(within(panel).getByText("Photos")).toBeVisible();
    expect(
      within(panel).getByRole("link", { name: "Poly Haven" }),
    ).toHaveAttribute("href", "https://polyhaven.com");
  });

  it("selects a 3D room or a photo by id", () => {
    const onChange = vi.fn();
    picker({ state: "ready" }, { onChange });
    open();
    fireEvent.click(
      screen.getByRole("radio", { name: "Meeting room (3D room)" }),
    );
    expect(onChange).toHaveBeenLastCalledWith("meeting-3d");
    fireEvent.click(screen.getByRole("radio", { name: "Balcony" }));
    expect(onChange).toHaveBeenLastCalledWith("balcony");
  });

  it("shows download progress on the button and in the panel", () => {
    picker({ state: "loading", progress: 0.42 });
    expect(screen.getByTestId("scene-progress-ring")).toBeInTheDocument();
    open();
    expect(screen.getByRole("status")).toHaveTextContent(
      "Loading the 3D room… 42%",
    );
  });

  it.each([
    ["unsupported", "This device shows a photo of the room instead of 3D."],
    ["error", "The 3D room couldn't load, so a photo is shown instead."],
    [
      "timeout",
      "The 3D room took too long to load, so a photo is shown instead.",
    ],
  ] as const)("explains a %s fallback to the still", (reason, text) => {
    picker({ state: "fallback", reason });
    expect(screen.queryByTestId("scene-progress-ring")).not.toBeInTheDocument();
    open();
    expect(screen.getByRole("status")).toHaveTextContent(text);
  });

  it("stays quiet for a photo and translates the room chrome", () => {
    picker({ state: "off" }, { value: "balcony", locale: "vi" });
    open("Chọn phông nền");
    expect(screen.getByRole("status")).toBeEmptyDOMElement();
    expect(
      screen.getByRole("radio", { name: "Phòng họp (phòng 3D)" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "Ban công" })).toHaveAttribute(
      "aria-checked",
      "true",
    );
  });
});
