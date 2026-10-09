"use client";

import { useEffect, useState } from "react";
import { motion, useMotionValue, useSpring } from "motion/react";

const viewBoxWidth = 1440;
const viewBoxHeight = 900;
const smoothEase = [0.22, 1, 0.36, 1] as const;

/** A soft cursor light that stays hidden over the hero polygon. */
export function HeroCursorGlow() {
  const pointerX = useMotionValue(-1000);
  const pointerY = useMotionValue(-1000);
  const x = useSpring(pointerX, { stiffness: 100, damping: 30, mass: 0.8 });
  const y = useSpring(pointerY, { stiffness: 100, damping: 30, mass: 0.8 });
  const [isVisible, setIsVisible] = useState(false);

  useEffect(() => {
    const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
    const finePointer = window.matchMedia("(pointer: fine)");
    const heroSurfaceRef =
      document.querySelector<HTMLElement>("[data-hero-shape]");
    const shape = heroSurfaceRef?.dataset.heroShape;
    const hitTestContextRef = document.createElement("canvas").getContext("2d");

    if (
      reducedMotion.matches ||
      !finePointer.matches ||
      !heroSurfaceRef ||
      !shape ||
      !hitTestContextRef
    ) {
      return;
    }

    const heroSurface: HTMLElement = heroSurfaceRef;
    const hitTestContext: CanvasRenderingContext2D = hitTestContextRef;
    const heroPath = new Path2D(shape);
    let lastPointer: { x: number; y: number } | null = null;
    let scrollFrame = 0;

    function updateVisibility(clientX: number, clientY: number) {
      const bounds = heroSurface.getBoundingClientRect();
      const withinHero =
        clientX >= bounds.left &&
        clientX <= bounds.right &&
        clientY >= bounds.top &&
        clientY <= bounds.bottom;

      const overHeroShape =
        withinHero &&
        hitTestContext.isPointInPath(
          heroPath,
          ((clientX - bounds.left) / bounds.width) * viewBoxWidth,
          ((clientY - bounds.top) / bounds.height) * viewBoxHeight,
        );

      const showGlow = !overHeroShape;
      setIsVisible((visible) => (visible === showGlow ? visible : showGlow));
    }

    function handlePointerMove(event: PointerEvent) {
      if (event.pointerType === "touch") return;

      lastPointer = { x: event.clientX, y: event.clientY };
      pointerX.set(event.clientX);
      pointerY.set(event.clientY);
      updateVisibility(event.clientX, event.clientY);
    }

    function handleScroll() {
      const pointer = lastPointer;
      if (!pointer) return;

      window.cancelAnimationFrame(scrollFrame);
      scrollFrame = window.requestAnimationFrame(() => {
        updateVisibility(pointer.x, pointer.y);
      });
    }

    function hideGlow() {
      setIsVisible(false);
    }

    window.addEventListener("pointermove", handlePointerMove, {
      passive: true,
    });
    document.addEventListener("scroll", handleScroll, {
      capture: true,
      passive: true,
    });
    document.documentElement.addEventListener("pointerleave", hideGlow);
    window.addEventListener("blur", hideGlow);

    return () => {
      window.removeEventListener("pointermove", handlePointerMove);
      document.removeEventListener("scroll", handleScroll, true);
      document.documentElement.removeEventListener("pointerleave", hideGlow);
      window.removeEventListener("blur", hideGlow);
      window.cancelAnimationFrame(scrollFrame);
    };
  }, [pointerX, pointerY]);

  return (
    <motion.div
      aria-hidden="true"
      animate={{ opacity: isVisible ? 1 : 0 }}
      className="pointer-events-none fixed top-0 left-0 z-50 size-[min(32rem,92vw)] -translate-x-1/2 -translate-y-1/2"
      initial={false}
      style={{
        x,
        y,
        background:
          "radial-gradient(circle, rgb(67 133 225 / 0.19) 0%, rgb(67 133 225 / 0.12) 22%, rgb(67 133 225 / 0.045) 48%, transparent 72%)",
      }}
      transition={{ duration: isVisible ? 0.42 : 0.28, ease: smoothEase }}
    />
  );
}
