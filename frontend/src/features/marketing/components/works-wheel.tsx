"use client";

import Image from "next/image";
import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from "react";
import { cn } from "@/lib/utils";
import { landingFocus } from "./rolecue-landing-styles";

export type WorksWheelItem = { title: string; image: string; alt: string };

// CrafterUI's ring → perspective drum geometry, tuned for five landscape covers.
const CARD_RATIO = 1.45;
const STEP = 40;
const DRUM = 2.22;
const LENS = 2.7;
const BOW = 1.82;
const CULL = 1.6;
const WHEEL_UNITS = 900;
const DRAG_UNITS = 420;
const SETTLE = 160;
const clamp = (value: number, low: number, high: number) =>
  Math.min(high, Math.max(low, value));
const radians = (degrees: number) => (degrees * Math.PI) / 180;

function place(
  ringDeg: number,
  drumDeg: number,
  ringR: number,
  drumR: number,
  bow: number,
  morph: number,
) {
  const bend = -bow * (1 - Math.cos(radians(drumDeg)));
  return `translateX(${morph * bend}px) rotateZ(${(1 - morph) * ringDeg}deg) translateY(${-(1 - morph) * ringR}px) rotateX(${morph * drumDeg}deg) translateZ(${morph * drumR}px)`;
}

export function WorksWheel({
  items,
  label,
}: {
  items: readonly WorksWheelItem[];
  label: string;
}) {
  const id = useId();
  const stageRef = useRef<HTMLDivElement>(null);
  const wheelRef = useRef<HTMLDivElement>(null);
  const cardRefs = useRef<(HTMLDivElement | null)[]>([]);
  const labelRef = useRef<HTMLDivElement>(null);
  const titleRef = useRef<HTMLDivElement>(null);
  const turn = useRef(0);
  const target = useRef(0);
  const drag = useRef<{ y: number; pointerId: number } | null>(null);
  const settling = useRef<ReturnType<typeof setTimeout> | null>(null);
  const requestDraw = useRef<() => void>(() => {});
  const [active, setActive] = useState(0);
  const [stage, setStage] = useState({ w: 0, h: 0 });
  const [reduced, setReduced] = useState(false);
  const count = items.length;
  const last = Math.max(count - 1, 0);

  useEffect(() => {
    const query = window.matchMedia("(prefers-reduced-motion: reduce)");
    const read = () => setReduced(query.matches);
    read();
    query.addEventListener("change", read);
    return () => query.removeEventListener("change", read);
  }, []);

  useEffect(() => {
    const element = stageRef.current;
    if (!element) return;
    const read = () =>
      setStage({ w: element.clientWidth, h: element.clientHeight });
    read();
    const observer = new ResizeObserver(read);
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  const metrics = useMemo(() => {
    const narrow = stage.w < 640;
    const cardW = Math.min(
      stage.h * (narrow ? 0.5 : 0.38) * CARD_RATIO,
      stage.w * (narrow ? 0.72 : 0.34),
    );
    const cardH = cardW / CARD_RATIO;
    const ringR = Math.min(cardH * 1.14, stage.h * 0.31, stage.w * 0.29);
    const ringScale = count
      ? clamp((((2 * Math.PI * ringR) / count) * 0.82) / (cardW || 1), 0.16, 1)
      : 1;
    return {
      cardW,
      cardH,
      ringR,
      ringScale,
      drumR: cardH * DRUM,
      bow: cardH * BOW,
      depth: cardH * LENS,
    };
  }, [stage, count]);

  const to = useCallback(
    (next: number) => {
      target.current = clamp(next, 0, last + 1);
      requestDraw.current();
    },
    [last],
  );

  useEffect(() => {
    if (!stage.h || !count) return;
    let frame = 0;
    let previousTime = 0;
    const { ringR, ringScale, drumR, bow } = metrics;

    const draw = (time: number) => {
      frame = 0;
      const elapsed = previousTime ? Math.min(time - previousTime, 32) : 16.67;
      previousTime = time;
      const gap = target.current - turn.current;
      if (reduced || Math.abs(gap) < 0.0005) turn.current = target.current;
      else turn.current += gap * (1 - Math.pow(0.88, elapsed / 16.67));

      const morph = reduced ? 1 : clamp(turn.current, 0, 1);
      const position = Math.max(0, turn.current - 1);
      if (wheelRef.current)
        wheelRef.current.style.transform = `translateZ(${-morph * drumR}px)`;

      for (let index = 0; index < count; index++) {
        const distance = index - position;
        const card = cardRefs.current[index];
        if (!card) continue;
        const hidden = reduced
          ? index !== Math.round(position)
          : morph > 0.5 && Math.abs(distance) > CULL;
        card.style.transform = reduced
          ? "none"
          : place(
              distance * (360 / count),
              distance * STEP,
              ringR,
              drumR,
              bow,
              morph,
            );
        card.style.opacity = hidden ? "0" : "1";
        card.style.visibility = hidden ? "hidden" : "visible";
        card.style.zIndex = String(Math.round(100 - Math.abs(distance) * 2));
        const face = card.firstElementChild;
        if (face instanceof HTMLElement)
          face.style.transform = `scale(${reduced ? 1 : ringScale + (1 - ringScale) * morph})`;
      }

      if (reduced && wheelRef.current)
        wheelRef.current.style.transform = "none";
      if (labelRef.current) labelRef.current.style.opacity = String(1 - morph);
      if (titleRef.current) titleRef.current.style.opacity = String(morph);
      const near = clamp(Math.round(position), 0, last);
      setActive((current) => (current === near ? current : near));
      // The reference loops forever. Here the wheel sleeps once it lands.
      if (target.current !== turn.current) frame = requestAnimationFrame(draw);
      else previousTime = 0;
    };

    requestDraw.current = () => {
      if (!frame) frame = requestAnimationFrame(draw);
    };
    requestDraw.current();
    return () => {
      cancelAnimationFrame(frame);
      requestDraw.current = () => {};
    };
  }, [metrics, stage.h, count, last, reduced]);

  useEffect(() => {
    const element = stageRef.current;
    if (!element || reduced) return;
    let gestureStart: number | null = null;
    const onWheel = (event: WheelEvent) => {
      if (event.ctrlKey || Math.abs(event.deltaX) > Math.abs(event.deltaY))
        return;
      const bounds = element.getBoundingClientRect();
      // Let the page reach the artwork before accepting scroll as a turn.
      const midpoint = bounds.top + bounds.height / 2;
      if (midpoint < innerHeight * 0.2 || midpoint > innerHeight * 0.8) return;
      const delta =
        event.deltaY *
        (event.deltaMode === 1
          ? 16
          : event.deltaMode === 2
            ? bounds.height
            : 1);
      const next = target.current + delta / WHEEL_UNITS;
      const movingInside =
        (delta > 0 && target.current < last + 1) ||
        (delta < 0 && target.current > 0);
      if (!movingInside) return;
      event.preventDefault();
      if (gestureStart === null) gestureStart = Math.round(target.current);
      to(next);
      if (settling.current) clearTimeout(settling.current);
      settling.current = setTimeout(() => {
        let nextNotch = Math.round(target.current);
        // A short mouse-wheel notch still advances one card; long gestures
        // retain their continuous movement across several cards.
        if (nextNotch === gestureStart) nextNotch += Math.sign(delta);
        to(nextNotch);
        gestureStart = null;
        settling.current = null;
      }, SETTLE);
    };
    element.addEventListener("wheel", onWheel, { passive: false });
    return () => {
      element.removeEventListener("wheel", onWheel);
      if (settling.current) clearTimeout(settling.current);
    };
  }, [to, last, reduced]);

  if (!count) return null;

  return (
    <div
      className="relative mx-auto h-[clamp(34rem,76svh,48rem)] w-full max-w-400 overflow-hidden text-(--rolecue-ink) max-sm:h-[34rem]"
      data-works-wheel=""
    >
      <div
        ref={stageRef}
        tabIndex={0}
        role="listbox"
        aria-label={label}
        aria-describedby={`${id}-instructions`}
        aria-activedescendant={`${id}-item-${active}`}
        className={cn(
          "absolute inset-0 cursor-grab select-none touch-pan-y active:cursor-grabbing max-sm:bottom-28 max-sm:overflow-hidden",
          landingFocus,
        )}
        style={{
          perspective: metrics.depth ? `${metrics.depth}px` : undefined,
        }}
        onPointerDown={(event) => {
          // Vertical touch gestures remain page scroll; touch users have the index.
          if (event.pointerType === "touch" || event.button !== 0) return;
          if (settling.current) clearTimeout(settling.current);
          drag.current = { y: event.clientY, pointerId: event.pointerId };
          event.currentTarget.setPointerCapture(event.pointerId);
        }}
        onPointerMove={(event) => {
          if (!drag.current) return;
          to(target.current + (drag.current.y - event.clientY) / DRAG_UNITS);
          drag.current.y = event.clientY;
        }}
        onPointerUp={(event) => {
          if (!drag.current) return;
          drag.current = null;
          if (event.currentTarget.hasPointerCapture(event.pointerId))
            event.currentTarget.releasePointerCapture(event.pointerId);
          to(target.current > 0 ? Math.max(1, Math.round(target.current)) : 0);
        }}
        onPointerCancel={() => {
          drag.current = null;
          to(Math.round(target.current));
        }}
        onLostPointerCapture={() => {
          drag.current = null;
        }}
        onKeyDown={(event) => {
          if (event.key === "ArrowDown" || event.key === "ArrowRight")
            to(Math.round(target.current) + 1);
          else if (event.key === "ArrowUp" || event.key === "ArrowLeft")
            to(Math.round(target.current) - 1);
          else if (event.key === "Home") to(0);
          else if (event.key === "End") to(last + 1);
          else return;
          event.preventDefault();
        }}
      >
        <div
          ref={wheelRef}
          className="absolute top-1/2 left-1/2 [transform-style:preserve-3d]"
        >
          {items.map((item, index) => (
            <div
              key={item.title}
              id={`${id}-item-${index}`}
              role="option"
              aria-label={item.title}
              aria-selected={index === active}
              ref={(node) => {
                cardRefs.current[index] = node;
              }}
              className="absolute [backface-visibility:hidden]"
              style={{
                width: metrics.cardW,
                height: metrics.cardH,
                marginLeft: -metrics.cardW / 2,
                marginTop: -metrics.cardH / 2,
              }}
            >
              <span className="relative block size-full overflow-hidden rounded-lg bg-(--rolecue-surface-soft) shadow-[0_18px_40px_-18px_rgb(38_38_38/22%)]">
                <Image
                  src={item.image}
                  alt={item.alt}
                  fill
                  sizes="(max-width: 640px) 72vw, 480px"
                  draggable={false}
                  className="pointer-events-none object-cover"
                />
              </span>
            </div>
          ))}
        </div>
      </div>

      <div
        ref={labelRef}
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 grid place-items-center max-sm:bottom-28"
      >
        <p className="max-w-[10ch] text-center text-[clamp(1.25rem,2.4vw,2rem)] leading-tight font-medium tracking-tight">
          {label}
        </p>
      </div>
      <div
        ref={titleRef}
        aria-hidden="true"
        className="pointer-events-none absolute top-1/2 left-[6%] max-w-[22%] -translate-y-1/2 text-[clamp(1.25rem,2.3vw,2rem)] leading-tight tracking-tight opacity-0 max-sm:top-auto max-sm:bottom-19 max-sm:left-0 max-sm:w-full max-sm:max-w-none max-sm:translate-y-0 max-sm:text-center"
      >
        {items[active].title}
      </div>
      <ol
        aria-label="Choose a practice moment"
        className="absolute top-[7%] right-[4%] text-right text-sm max-sm:top-auto max-sm:inset-x-4 max-sm:bottom-4 max-sm:flex max-sm:justify-center max-sm:gap-2"
      >
        {items.map((item, index) => (
          <li key={item.title}>
            <button
              type="button"
              aria-label={`Show ${item.title}`}
              aria-pressed={index === active}
              onClick={() => to(index + 1)}
              className={cn(
                landingFocus,
                "min-h-11 cursor-pointer text-(--rolecue-ink-muted) transition-colors hover:text-(--rolecue-ink) max-sm:size-11 max-sm:rounded-full max-sm:border max-sm:border-(--rolecue-border)",
                index === active &&
                  "font-semibold text-(--rolecue-ink)! max-sm:bg-(--rolecue-surface)",
              )}
            >
              <span className="hidden tabular-nums max-sm:inline">
                {String(index + 1).padStart(2, "0")}
              </span>
              <span className="max-sm:hidden">{item.title}</span>
            </button>
          </li>
        ))}
      </ol>
      <p id={`${id}-instructions`} className="sr-only">
        Use the arrow keys or choose a moment to explore the practice loop.
      </p>
      <p
        aria-hidden="true"
        className="pointer-events-none absolute bottom-6 inset-x-0 text-center text-xs text-(--rolecue-ink-subtle) max-sm:hidden"
      >
        {reduced ? "Choose a moment" : "Scroll or drag to explore"}
      </p>
    </div>
  );
}
