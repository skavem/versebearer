// Gradient model shared by the editor and its preview. Stored on a theme as
// JSON in the `bgGradient` field; the receiver rebuilds the same CSS from it.

export type GradientStop = { color: string; pos: number };
export type GradientType = "linear" | "radial" | "conic";
export type Gradient = { type: GradientType; angle: number; stops: GradientStop[] };

const HEX = /^#[0-9a-fA-F]{3,8}$/;

export function defaultGradient(): Gradient {
  return {
    type: "linear",
    angle: 135,
    stops: [
      { color: "#1e3a8a", pos: 0 },
      { color: "#9333ea", pos: 100 },
    ],
  };
}

export function parseGradient(json: string): Gradient {
  if (json) {
    try {
      const g = JSON.parse(json) as Gradient;
      if (g && Array.isArray(g.stops) && g.stops.length >= 1) {
        return {
          type: g.type === "radial" || g.type === "conic" ? g.type : "linear",
          angle: Number.isFinite(g.angle) ? g.angle : 135,
          stops: g.stops.map((s) => ({
            color: HEX.test(s.color) ? s.color : "#000000",
            pos: clamp(Number(s.pos) || 0, 0, 100),
          })),
        };
      }
    } catch {
      // fall through to default
    }
  }
  return defaultGradient();
}

export function stringifyGradient(g: Gradient): string {
  return JSON.stringify(g);
}

export function gradientToCss(g: Gradient): string {
  const stops = g.stops.map((s) => `${s.color} ${clamp(s.pos, 0, 100)}%`).join(", ");
  switch (g.type) {
    case "radial":
      return `radial-gradient(circle, ${stops})`;
    case "conic":
      return `conic-gradient(from ${g.angle}deg, ${stops})`;
    case "linear":
    default:
      return `linear-gradient(${g.angle}deg, ${stops})`;
  }
}

function clamp(n: number, lo: number, hi: number): number {
  return Math.min(hi, Math.max(lo, n));
}

export const GRADIENT_PRESETS: { name: string; gradient: Gradient }[] = [
  {
    name: "Ночь",
    gradient: { type: "linear", angle: 135, stops: [{ color: "#0f172a", pos: 0 }, { color: "#1e3a8a", pos: 100 }] },
  },
  {
    name: "Закат",
    gradient: { type: "linear", angle: 135, stops: [{ color: "#7c2d12", pos: 0 }, { color: "#b91c1c", pos: 50 }, { color: "#f59e0b", pos: 100 }] },
  },
  {
    name: "Аметист",
    gradient: { type: "linear", angle: 160, stops: [{ color: "#312e81", pos: 0 }, { color: "#9333ea", pos: 100 }] },
  },
  {
    name: "Изумруд",
    gradient: { type: "linear", angle: 135, stops: [{ color: "#064e3b", pos: 0 }, { color: "#059669", pos: 100 }] },
  },
  {
    name: "Океан",
    gradient: { type: "radial", angle: 0, stops: [{ color: "#0e7490", pos: 0 }, { color: "#0f172a", pos: 100 }] },
  },
  {
    name: "Сияние",
    gradient: { type: "conic", angle: 90, stops: [{ color: "#1e293b", pos: 0 }, { color: "#475569", pos: 50 }, { color: "#1e293b", pos: 100 }] },
  },
  {
    name: "Чёрный",
    gradient: { type: "linear", angle: 0, stops: [{ color: "#000000", pos: 0 }, { color: "#000000", pos: 100 }] },
  },
];
