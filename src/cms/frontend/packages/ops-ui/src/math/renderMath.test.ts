// F053 T033 — F053-AC-17 (TC-UI-012, TC-UI-013): LaTeX → SVG `currentColor` + speech text, math_id sha256,
// forbidden commands (\href, \require, \html…, macros) and the latex_max_length / latex_max_depth / math_svg_max limits.
import { describe, expect, it } from 'vitest';
import { checkLatex, mathId, MathRenderError, monochrome, renderMath } from './renderMath';
import { forbiddenCommand, LATEX_MAX_DEPTH, LATEX_MAX_LENGTH, latexDepth } from './mathjaxConfig';

describe('renderMath (F053-AC-17)', () => {
  it('renders a fraction to a self-contained monochrome SVG with a Vietnamese speech text', async () => {
    const r = await renderMath(String.raw`\frac{3}{4}`, { language: 'vi' });
    expect(r.svg.startsWith('<svg')).toBe(true);
    expect(r.svg).toContain('currentColor');
    expect(r.svg).not.toMatch(/fill="(?!none|currentColor)[^"]*"/);
    expect(r.svg).not.toContain('<use'); // fontCache: 'none' → paths inline, nothing shared
    expect(r.speech_text).toBe('phân số 3 trên 4');
    expect(r.math_id).toMatch(/^[0-9a-f]{64}$/);
    expect(r.math_id).toBe(await mathId(String.raw`\frac{3}{4}`, false));
  });

  it('renders chemistry (mhchem) and physics formulas; English speech for English content', async () => {
    const ce = await renderMath(String.raw`\ce{2H2 + O2 -> 2H2O}`, { language: 'vi' });
    expect(ce.svg).toContain('<svg');
    expect(ce.speech_text).toContain('tạo thành');
    const en = await renderMath(String.raw`\sqrt{x^2+1}`, { language: 'en' });
    expect(en.speech_text).toBe('the square root of x squared plus 1');
    const deg = await renderMath(String.raw`30^\circ`, { language: 'vi' });
    expect(deg.speech_text).toBe('30 độ');
  });

  it('block and inline of the same LaTeX have different ids', async () => {
    expect(await mathId('x', true)).not.toBe(await mathId('x', false));
  });

  it('rejects \\href, \\require, \\html…, macro definitions and colors before rendering', async () => {
    for (const bad of [String.raw`\href{https://x}{y}`, String.raw`\require{html}`, String.raw`\htmlClass{a}{b}`, String.raw`\def\x{1}`, String.raw`\newcommand{\a}{b}`, String.raw`\color{red}{x}`, String.raw`\unicode{x41}`]) {
      await expect(renderMath(bad)).rejects.toMatchObject({ code: 'MATH_FORBIDDEN_COMMAND' });
    }
    expect(forbiddenCommand(String.raw`\hrefx`)).toBeNull(); // not the same command
    expect(forbiddenCommand(String.raw`\frac{1}{2}`)).toBeNull();
  });

  it('raises MATH_SYNTAX on a syntax error (no red error box rendered)', async () => {
    const err = await renderMath(String.raw`\frac{1`).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(MathRenderError);
    expect((err as MathRenderError).code).toBe('MATH_SYNTAX');
  });

  it('enforces latex_max_length and latex_max_depth', () => {
    expect(checkLatex('x'.repeat(LATEX_MAX_LENGTH + 1))?.code).toBe('MATH_TOO_LONG');
    expect(checkLatex('x'.repeat(LATEX_MAX_LENGTH))).toBeNull();
    const deep = '{'.repeat(LATEX_MAX_DEPTH + 1) + 'x' + '}'.repeat(LATEX_MAX_DEPTH + 1);
    expect(latexDepth(deep)).toBe(LATEX_MAX_DEPTH + 1);
    expect(checkLatex(deep)?.code).toBe('MATH_TOO_DEEP');
    expect(latexDepth(String.raw`\left(\frac{a}{b}\right)`)).toBe(2);
    expect(latexDepth(String.raw`\{1\}`)).toBe(0);
    expect(checkLatex('  ')?.code).toBe('MATH_EMPTY');
  });

  it('monochrome() rewrites any paint to currentColor', () => {
    expect(monochrome('<g fill="red" stroke="#000"><path fill="none"/></g>')).toBe('<g fill="currentColor" stroke="currentColor"><path fill="none"/></g>');
  });
});
