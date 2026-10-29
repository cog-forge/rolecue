import { render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { RoomTranscript } from "@/features/interview/components/room-transcript";
it("renders only supplied questions and STT answers and updates a partial turn in place", () => {
  const expand = vi.fn();
  const entries = [
    {
      id: "q1",
      speaker: "interviewer" as const,
      text: "How did you diagnose the latency?",
    },
    {
      id: "a1",
      speaker: "candidate" as const,
      text: "I measured the",
      partial: true,
    },
  ];
  const view = render(
    <RoomTranscript expanded={false} onExpand={expand} entries={entries} />,
  );
  expect(screen.getByText("How did you diagnose the latency?")).toBeVisible();
  expect(screen.getByText("Transcribing…")).toBeVisible();
  expect(screen.getByRole("log")).toHaveAttribute("aria-relevant", "additions");
  view.rerender(
    <RoomTranscript
      expanded={false}
      onExpand={expand}
      entries={[
        entries[0],
        {
          ...entries[1],
          text: "I measured the slow database query.",
          partial: false,
        },
      ]}
    />,
  );
  expect(screen.getByText("I measured the slow database query.")).toBeVisible();
  expect(screen.queryByText("Transcribing…")).not.toBeInTheDocument();
  expect(
    view.container.querySelectorAll('[data-message-id="a1"]'),
  ).toHaveLength(1);
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  expect(
    screen.queryByText("Live captions are unavailable in this preview."),
  ).not.toBeInTheDocument();
});
