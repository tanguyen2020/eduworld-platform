// Color helpers for the token pipeline (F005): hex parsing, WCAG 2.1 relative luminance and contrast.
// https://www.w3.org/TR/WCAG21/#dfn-relative-luminance

const HEX = /^#([0-9a-fA-F]{6})([0-9a-fA-F]{2})?$/;

export function isHex(value) {
  return typeof value === 'string' && HEX.test(value);
}

/** '#RRGGBB' | '#RRGGBBAA' → {r,g,b,a} with r,g,b in 0..255 and a in 0..1. */
export function parseHex(value) {
  const m = HEX.exec(value);
  if (!m) throw new Error(`not a #RRGGBB[AA] color: ${value}`);
  const n = parseInt(m[1], 16);
  return {
    r: (n >> 16) & 255,
    g: (n >> 8) & 255,
    b: n & 255,
    a: m[2] ? parseInt(m[2], 16) / 255 : 1,
  };
}

function channel(c) {
  const s = c / 255;
  return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
}

export function relativeLuminance(hex) {
  const { r, g, b } = parseHex(hex);
  return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b);
}

/** WCAG 2.1 contrast ratio of two opaque colors, rounded down to 2 decimals (never rounds a fail up). */
export function contrastRatio(fg, bg) {
  const a = relativeLuminance(fg);
  const b = relativeLuminance(bg);
  const [hi, lo] = a > b ? [a, b] : [b, a];
  return Math.floor(((hi + 0.05) / (lo + 0.05)) * 100) / 100;
}

/** Minimum ratio per level: `normal` text 4.5:1, `large` text and graphical objects 3:1 (WCAG 2.1 AA). */
export const WCAG_AA = Object.freeze({ normal: 4.5, large: 3, graphic: 3 });

/** Godot `Color(r, g, b, a)` literal with up to 4 decimals. */
export function godotColor(hex) {
  const { r, g, b, a } = parseHex(hex);
  const f = (x) => String(Math.round(x * 10000) / 10000);
  return `Color(${f(r / 255)}, ${f(g / 255)}, ${f(b / 255)}, ${f(a)})`;
}

export function normalizeHex(value) {
  return value.toUpperCase();
}
