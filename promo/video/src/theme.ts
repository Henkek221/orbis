import { Easing } from "remotion";

// Every value here comes from SPEC.md, not from habit.
export const FPS = 30;
export const W = 1920;
export const H = 1080;

export const color = {
  bg: "#050608",
  terminal: "#0b0f1a",
  text: "#f5f5f7",
  muted: "#86868b",
  blue: "#0a84ff",
  green: "#30d158",
};

export const font = {
  display: '-apple-system, "SF Pro Display", "Helvetica Neue", sans-serif',
  mono: '"SF Mono", ui-monospace, Menlo, monospace',
};

// Named curves from the spec. There is deliberately no default.
export const curve = {
  // Tile/terminal element lands: fast in, settles, no bounce.
  land: Easing.bezier(0.16, 1, 0.3, 1),
  // Camera and panels glide: slow start, slow end.
  glide: Easing.bezier(0.65, 0, 0.35, 1),
  // Leaving the frame: accelerate out.
  exit: Easing.bezier(0.7, 0, 0.84, 0),
};

export const clamp = { extrapolateLeft: "clamp", extrapolateRight: "clamp" } as const;
