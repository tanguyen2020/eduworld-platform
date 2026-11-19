// LaTeX delimiters inside content text (F053-AC-16, ADMIN_PORTAL §1.4): inline `\( … \)`, block `\[ … \]`. Pure.

export type MathSegment = { kind: 'text'; value: string } | { kind: 'math'; latex: string; display: boolean; start: number; end: number };

const DELIM = /\\\(([\s\S]*?)\\\)|\\\[([\s\S]*?)\\\]/g;

/** Splits a text into text and formula segments (an unclosed delimiter stays text). */
export function splitMath(text: string): MathSegment[] {
  const out: MathSegment[] = [];
  let last = 0;
  for (const m of text.matchAll(DELIM)) {
    const start = m.index ?? 0;
    if (start > last) out.push({ kind: 'text', value: text.slice(last, start) });
    const inline = m[1];
    out.push({ kind: 'math', latex: (inline ?? m[2] ?? '').trim(), display: inline === undefined, start, end: start + m[0].length });
    last = start + m[0].length;
  }
  if (last < text.length) out.push({ kind: 'text', value: text.slice(last) });
  return out;
}

/** `\( latex \)` or `\[ latex \]`. */
export function wrapLatex(latex: string, display: boolean): string {
  return display ? `\\[${latex}\\]` : `\\(${latex}\\)`;
}

export function hasMath(text: string | null | undefined): boolean {
  return !!text && splitMath(text).some((s) => s.kind === 'math');
}
