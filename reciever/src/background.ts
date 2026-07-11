import type { IBackdrop } from "./types";

// Gradient stored on a theme as JSON: { type, angle, stops:[{color,pos}] }.
// The receiver rebuilds the CSS from this structured data (never injects raw
// CSS), validating colors and clamping positions.
type GradientStop = { color: string; pos: number };
type Gradient = { type: string; angle: number; stops: GradientStop[] };

const HEX = /^#[0-9a-fA-F]{3,8}$/;

function clamp(n: number, lo: number, hi: number): number {
  return Math.min(hi, Math.max(lo, n));
}

export function gradientJsonToCss(json: string): string | null {
  if (!json) return null;
  let g: Gradient;
  try {
    g = JSON.parse(json);
  } catch {
    return null;
  }
  if (!g || !Array.isArray(g.stops) || g.stops.length === 0) return null;

  const stops = g.stops
    .filter((s) => typeof s.color === "string" && HEX.test(s.color))
    .map((s) => `${s.color} ${clamp(Number(s.pos) || 0, 0, 100)}%`)
    .join(", ");
  if (!stops) return null;

  const angle = Number.isFinite(g.angle) ? g.angle : 0;
  switch (g.type) {
    case "radial":
      return `radial-gradient(circle, ${stops})`;
    case "conic":
      return `conic-gradient(from ${angle}deg, ${stops})`;
    case "linear":
    default:
      return `linear-gradient(${angle}deg, ${stops})`;
  }
}

// Build the CSS `background` value for the full-screen backdrop, or null when
// there is no backdrop (bgType "none"/unset or an incomplete config).
export function backdropCss(b: IBackdrop): string | null {
  switch (b.bgType) {
    case "gradient":
      return gradientJsonToCss(b.bgGradient);
    case "image":
      if (b.bgImageId == null) return null;
      return `center / cover no-repeat url("/image/${b.bgImageId}")`;
    default:
      return null;
  }
}
