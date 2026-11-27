// F028 research R2 (spike, TC-MATH-001 subset): the LaTeX Pandoc 3.6.4 writes for the OMML catalog of the spike
// (src/backend/modules/learning/internal/infrastructure/docximport/testdata/pandoc-3.6.4-formulas.json) passes the
// pre-checks of the portal and renders with its MathJax 4, so a Word formula needs no rewriting after import.
import { describe, expect, it } from 'vitest';
import { checkLatex, renderMath, type MathRenderError } from '@eduworld/ops-ui/content';

const PANDOC_LATEX: Record<string, string> = {
  fraction: String.raw`\frac{3}{4}`,
  mixed_fraction: String.raw`2\frac{1}{3}`,
  nested_fraction: String.raw`\frac{1}{1 + \frac{1}{x}}`,
  sqrt: String.raw`\sqrt{x + 1}`,
  cube_root: String.raw`\sqrt[3]{8}`,
  superscript: String.raw`x^{2} + y^{3}`,
  subscript: String.raw`a_{n} = a_{1} + (n - 1)d`,
  chem_subscript: String.raw`H_{2}O`,
  matrix_2x2: String.raw`\begin{pmatrix}
1 & 2 \\
3 & 4
\end{pmatrix}`,
  sum: String.raw`\sum_{i = 1}^{n}i`,
  integral: String.raw`\int_{0}^{1}{x^{2}dx}`,
  vector: String.raw`\overrightarrow{AB}`,
  overbar: String.raw`\overline{abc}`,
  sin: String.raw`\sin x`,
  limit: String.raw`\lim_{x \rightarrow 0}\frac{\sin x}{x}`,
  system: String.raw`\left\{ \begin{array}{r}
x + y = 5 \\
x - y = 1
\end{array} \right.\ `,
  relations: String.raw`a \leq b,\ c \geq d,\ e \neq f,\ x \in {\mathbb{R}},\ A \subset B`,
  greek_degree: String.raw`\alpha + \beta = 90{^\circ}`,
  pm_times_div: String.raw`\pm 3 \times 4 \div 2`,
  text_in_math: String.raw`v = \frac{\text{quãng đường}}{\text{thời gian}}`,
  parentheses: String.raw`\left( \frac{a}{b} \right)^{2}`,
  percent: String.raw`25\%`,
};

describe('Pandoc OMML → LaTeX renders with MathJax 4 (F028 research R2)', () => {
  for (const [name, latex] of Object.entries(PANDOC_LATEX)) {
    it(name, async () => {
      expect(checkLatex(latex)).toBeNull();
      const r = await renderMath(latex, { display: false, language: 'vi' }).catch((e: MathRenderError) => {
        throw new Error(`${e.code}: ${e.detail ?? ''}`);
      });
      expect(r.svg.startsWith('<svg')).toBe(true);
      expect(r.svg).not.toContain('merror');
      expect(r.speech_text.length).toBeGreaterThan(0);
    });
  }
});
