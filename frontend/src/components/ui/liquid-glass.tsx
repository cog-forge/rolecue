"use client";

import * as React from "react";
import { Slot } from "radix-ui";

import { cn } from "@/lib/utils";
import {
  createDisplacementMap,
  createSpecularMap,
  DEFAULT_IOR,
  DEFAULT_THICKNESS_RATIO,
  preloadDisplacementMap,
  supportsLiquidGlass,
  type GlassSurface,
} from "@/lib/opaline/glass-refraction";

/** Optical settings shared by every glass surface. */
type GlassSettings = {
  /** Index of refraction: 1 is air, 1.5 is window glass, 2.4 is diamond. */
  ior?: number;
  /** Cross-section of the rim: `squircle` (Apple), `circle`, `concave` or `lip`. */
  surface?: GlassSurface;
  /** Height of the glass as a multiple of the bezel width (default 1.4). Thicker glass bends light further. */
  thickness?: number;
  /** Explicit refraction strength in px; overrides the value computed from the optics. */
  refraction?: number;
  /** Width of the refracting rim in px. Defaults to ~40% of the shortest side. */
  bezel?: number;
  /** Backdrop blur in px. Liquid glass bends light rather than blurring it — keep this small. */
  blur?: number;
  /** Backdrop saturation multiplier. */
  saturation?: number;
  /** Chromatic dispersion at the rim (0 – 1). */
  dispersion?: number;
  /** Strength of the specular highlight along the rim (0 – 1). */
  specular?: number;
  /** Direction the light comes from, in degrees (0 = right, -90 = top). */
  lightAngle?: number;
  /** Overrides the surface tint colour (defaults to `--glass-tint`). */
  tint?: string;
  /** `clear` is fully transparent; `frosted` adds a light tint and a touch of blur for legible text. */
  variant?: "clear" | "frosted";
  /** Draw the soft outer shadow. */
  shadow?: boolean;
};

type LiquidGlassProps = React.ComponentProps<"div"> &
  GlassSettings & {
    /** Render the glass onto its only child element instead of a div. */
    asChild?: boolean;
  };

const GlassContext = React.createContext<GlassSettings>({});

/**
 * Sets default optics for every glass surface inside it. Props set directly
 * on a component still win.
 *
 * ```tsx
 * <LiquidGlassProvider ior={1.8} surface="lip" specular={0.5}>
 *   <App />
 * </LiquidGlassProvider>
 * ```
 */
function LiquidGlassProvider({
  children,
  ior,
  surface,
  thickness,
  refraction,
  bezel,
  blur,
  saturation,
  dispersion,
  specular,
  lightAngle,
  tint,
  variant,
  shadow,
}: GlassSettings & { children?: React.ReactNode }) {
  const parent = React.useContext(GlassContext);
  const value = React.useMemo(
    () =>
      stripUndefined({
        ...parent,
        ior,
        surface,
        thickness,
        refraction,
        bezel,
        blur,
        saturation,
        dispersion,
        specular,
        lightAngle,
        tint,
        variant,
        shadow,
      }),
    [
      parent,
      ior,
      surface,
      thickness,
      refraction,
      bezel,
      blur,
      saturation,
      dispersion,
      specular,
      lightAngle,
      tint,
      variant,
      shadow,
    ],
  );
  return (
    <GlassContext.Provider value={value}>{children}</GlassContext.Provider>
  );
}

function stripUndefined<T extends object>(value: T): Partial<T> {
  const result: Partial<T> = {};
  for (const key in value) {
    if (value[key] !== undefined) result[key] = value[key];
  }
  return result;
}

/** The glass settings from the nearest `LiquidGlassProvider`. */
function useGlassSettings() {
  return React.useContext(GlassContext);
}

const noSubscription = () => () => {};

type MapEntry = { url: string; scale: number; specular: string; key: number };

function useSize(ref: React.RefObject<HTMLElement | null>) {
  const [size, setSize] = React.useState({ width: 0, height: 0, radius: 0 });

  React.useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const measure = () => {
      const width = el.offsetWidth;
      const height = el.offsetHeight;
      const radius = Math.min(
        parseFloat(getComputedStyle(el).borderTopLeftRadius) || 0,
        width / 2,
        height / 2,
      );
      setSize((s) =>
        s.width === width && s.height === height && s.radius === radius
          ? s
          : { width, height, radius },
      );
    };
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, [ref]);

  return size;
}

function GlassFilter({
  id,
  map,
  width,
  height,
  blur,
  scale,
  dispersion,
  saturation,
  specularMap,
  specular,
}: {
  id: string;
  map: string;
  specularMap: string;
  specular: number;
  width: number;
  height: number;
  blur: number;
  scale: number;
  dispersion: number;
  saturation: number;
}) {
  const displace = (k: number, result: string) => (
    <feDisplacementMap
      in="blur"
      in2="map"
      scale={scale * k}
      xChannelSelector="R"
      yChannelSelector="G"
      result={result}
    />
  );

  return (
    <filter
      id={id}
      x="0"
      y="0"
      width={width}
      height={height}
      filterUnits="userSpaceOnUse"
      colorInterpolationFilters="sRGB"
    >
      <feGaussianBlur in="SourceGraphic" stdDeviation={blur} result="blur" />
      <feImage
        href={map}
        x="0"
        y="0"
        width={width}
        height={height}
        preserveAspectRatio="none"
        result="map"
      />
      {dispersion > 0 ? (
        <>
          {displace(1, "dr")}
          <feColorMatrix
            in="dr"
            type="matrix"
            values="1 0 0 0 0  0 0 0 0 0  0 0 0 0 0  0 0 0 1 0"
            result="r"
          />
          {displace(1 - dispersion, "dg")}
          <feColorMatrix
            in="dg"
            type="matrix"
            values="0 0 0 0 0  0 1 0 0 0  0 0 0 0 0  0 0 0 1 0"
            result="g"
          />
          {displace(1 - dispersion * 2, "db")}
          <feColorMatrix
            in="db"
            type="matrix"
            values="0 0 0 0 0  0 0 0 0 0  0 0 1 0 0  0 0 0 1 0"
            result="b"
          />
          <feBlend in="r" in2="g" mode="screen" result="rg" />
          <feBlend in="rg" in2="b" mode="screen" result="refracted" />
        </>
      ) : (
        displace(1, "refracted")
      )}
      <feColorMatrix
        in="refracted"
        type="saturate"
        values={String(saturation)}
        result="saturated"
      />
      {specular > 0 && specularMap ? (
        <>
          <feImage
            href={specularMap}
            x="0"
            y="0"
            width={width}
            height={height}
            preserveAspectRatio="none"
            result="specularMap"
          />
          <feComponentTransfer in="specularMap" result="specular">
            <feFuncA type="linear" slope={specular} />
          </feComponentTransfer>
          <feComposite in="specular" in2="saturated" operator="over" />
        </>
      ) : null}
    </filter>
  );
}

function LiquidGlass({
  ref,
  className,
  style,
  children,
  asChild = false,
  ...rest
}: LiquidGlassProps) {
  const settings = useGlassSettings();
  const {
    ior = settings.ior ?? DEFAULT_IOR,
    surface = settings.surface ?? "squircle",
    thickness = settings.thickness,
    refraction = settings.refraction,
    bezel = settings.bezel,
    blur = settings.blur,
    saturation = settings.saturation ?? 1.6,
    dispersion = settings.dispersion ?? 0.12,
    specular = settings.specular ?? 0.2,
    lightAngle = settings.lightAngle ?? -60,
    tint = settings.tint,
    variant = settings.variant ?? "clear",
    shadow = settings.shadow ?? true,
    ...props
  } = rest;
  const Comp = asChild ? Slot.Root : "div";
  const innerRef = React.useRef<HTMLDivElement>(null);
  React.useImperativeHandle(ref, () => {
    const element = innerRef.current;
    if (!element) throw new Error("Glass surface is not mounted");
    return element;
  });

  const id = "lg" + React.useId().replace(/[^a-zA-Z0-9_-]/g, "");
  const enabled = React.useSyncExternalStore(
    noSubscription,
    supportsLiquidGlass,
    () => false,
  );

  const { width, height, radius } = useSize(innerRef);
  const rim = bezel ?? Math.max(6, Math.min(Math.min(width, height) * 0.4, 40));
  const depth = rim * (thickness ?? DEFAULT_THICKNESS_RATIO);
  const blurPx = blur ?? (variant === "frosted" ? 2 : 0.5);

  // The first map is built synchronously before the first paint, so the
  // surface is refracting from its very first frame. Later maps (on resize)
  // are debounced and double-buffered: the new map is mounted in its own
  // filter and only becomes active once decoded, while the current one is
  // stretched to the new size — the glass never flashes empty.
  const [current, setCurrent] = React.useState<MapEntry | null>(null);
  const [next, setNext] = React.useState<MapEntry | null>(null);
  const counter = React.useRef(0);
  const currentUrl = React.useRef("");
  const hasMap = current !== null;

  const build = React.useCallback(() => {
    const geometry = { width, height, radius, bezel: rim };
    const map = createDisplacementMap({
      ...geometry,
      surface,
      ior,
      thickness: depth,
    });
    if (!map.url) return null;
    const spec =
      specular > 0
        ? createSpecularMap({ ...geometry, surface, angle: lightAngle })
        : "";
    return { ...map, specular: spec };
  }, [width, height, radius, rim, surface, ior, depth, specular, lightAngle]);

  React.useLayoutEffect(() => {
    if (hasMap || !enabled || width === 0 || height === 0) return;
    const map = build();
    if (!map) return;
    currentUrl.current = map.url + map.specular;
    setCurrent({ ...map, key: ++counter.current });
    void preloadDisplacementMap(map.url);
  }, [enabled, width, height, hasMap, build]);

  React.useEffect(() => {
    if (!hasMap || !enabled || width === 0 || height === 0) return;
    let cancelled = false;
    const timer = setTimeout(() => {
      const map = build();
      if (!map) return;
      const id = map.url + map.specular;
      if (id === currentUrl.current) {
        // Same image, but the scale may differ (e.g. an explicit refraction).
        setCurrent((c) =>
          c && c.scale !== map.scale ? { ...c, scale: map.scale } : c,
        );
        return;
      }
      const entry = { ...map, key: ++counter.current };
      setNext(entry);
      Promise.all([
        preloadDisplacementMap(map.url),
        map.specular ? preloadDisplacementMap(map.specular) : null,
      ]).then(() => {
        if (cancelled) return;
        currentUrl.current = id;
        setCurrent(entry);
        setNext(null);
      });
    }, 140);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [enabled, width, height, hasMap, build]);

  const filterId = (entry: MapEntry) => `${id}-${entry.key}`;
  const backdrop = current
    ? `url(#${filterId(current)})`
    : // Safari / Firefox can't refract the backdrop; a light, saturated
      // frost is the closest match.
      `blur(${blur ?? (variant === "frosted" ? 10 : 5)}px) saturate(${saturation + 0.2}) brightness(1.04)`;
  const filters = [current, next].filter((e): e is MapEntry => e !== null);

  return (
    <Comp
      ref={innerRef}
      data-slot="liquid-glass"
      data-refracting={current ? "" : undefined}
      className={cn("relative isolate", className)}
      style={style}
      {...props}
    >
      <span
        aria-hidden
        data-slot="liquid-glass-backdrop"
        className="pointer-events-none absolute inset-0 -z-10 overflow-hidden rounded-[inherit]"
        style={{ backdropFilter: backdrop, WebkitBackdropFilter: backdrop }}
      />
      <span
        aria-hidden
        data-slot="liquid-glass-tint"
        className={cn(
          "pointer-events-none absolute inset-0 -z-10 rounded-[inherit]",
          variant === "frosted"
            ? "bg-(--glass-tint-frosted)"
            : "bg-(--glass-tint)",
        )}
        style={tint ? { background: tint } : undefined}
      />
      <span
        aria-hidden
        data-slot="liquid-glass-rim"
        className={cn(
          "pointer-events-none absolute inset-0 -z-10 rounded-[inherit] shadow-(--glass-rim)",
          shadow && "[box-shadow:var(--glass-rim),var(--glass-shadow)]",
        )}
      />
      {filters.length ? (
        <svg
          aria-hidden
          width="0"
          height="0"
          className="pointer-events-none absolute size-0"
        >
          {filters.map((entry) => (
            <GlassFilter
              key={entry.key}
              id={filterId(entry)}
              map={entry.url}
              width={width}
              height={height}
              blur={blurPx}
              scale={refraction ?? entry.scale}
              dispersion={dispersion}
              saturation={saturation}
              specularMap={entry.specular}
              specular={specular}
            />
          ))}
        </svg>
      ) : null}
      <Slot.Slottable>{children}</Slot.Slottable>
    </Comp>
  );
}

export {
  LiquidGlass,
  LiquidGlassProvider,
  useGlassSettings,
  type GlassSettings,
  type LiquidGlassProps,
};
