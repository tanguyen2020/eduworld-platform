// MathJax 4 configuration of the portal (ADR-0055, CONTENT_MODEL §9.1, SECURITY_ARCHITECTURE §8b): LaTeX is
// untrusted data. Only the TeX packages `base`, `ams`, `mhchem` are loaded (no `require`, `html`, `unicode`,
// `newcommand`, `configmacros`), and commands that link, style, define macros or load extensions are rejected before
// MathJax sees them (`base` itself still knows `\href`, `\class`…). The server checks again (F027).
// Limits are CONFIG_DEFAULTS `latex_max_length` / `latex_max_depth` / `math_svg_max` (scope "code", ADR-0055).

/** CONFIG_DEFAULTS `latex_max_length` (characters) — same value as OpenAPI `math_assets[].latex.maxLength`. */
export const LATEX_MAX_LENGTH = 2000;
/** CONFIG_DEFAULTS `latex_max_depth` (brace / \left nesting). */
export const LATEX_MAX_DEPTH = 20;
/** CONFIG_DEFAULTS `math_svg_max` (bytes) — same value as OpenAPI `math_assets[].svg.maxLength`. */
export const MATH_SVG_MAX = 262_144;
/** OpenAPI `math_assets[].speech_text.maxLength`. */
export const SPEECH_TEXT_MAX = 1000;

export const TEX_PACKAGES = ['base', 'ams', 'mhchem'] as const;

/**
 * Commands never allowed in content LaTeX: links/HTML attributes (`\href`, `\url`, `\class`, `\cssId`, `\style`,
 * `\data`, `\html…`), extension loading (`\require`), macro definitions (`\def`, `\newcommand`, `\let`…), raw
 * characters/tokens (`\unicode`, `\mmlToken`), interactive toggles and colors (SVG must stay monochrome).
 */
export const FORBIDDEN_COMMANDS = [
  'href',
  'url',
  'class',
  'cssId',
  'style',
  'data',
  'require',
  'def',
  'gdef',
  'edef',
  'xdef',
  'let',
  'newcommand',
  'renewcommand',
  'providecommand',
  'DeclareMathOperator',
  'newenvironment',
  'renewenvironment',
  'unicode',
  'mmlToken',
  'toggle',
  'mathtip',
  'texttip',
  'color',
  'textcolor',
  'colorbox',
  'fcolorbox',
  'definecolor',
  'bbox',
  'Bbox',
  'input',
  'include',
] as const;

const FORBIDDEN_RE = new RegExp(`\\\\(?:${FORBIDDEN_COMMANDS.join('|')}|html[A-Za-z]*)(?![A-Za-z])`);

/** First forbidden command of a LaTeX string (`\href`…), or null. */
export function forbiddenCommand(latex: string): string | null {
  const m = FORBIDDEN_RE.exec(latex);
  return m ? m[0] : null;
}

/** Deepest nesting of `{…}` groups and `\left…\right` pairs (escaped braces `\{` do not count). */
export function latexDepth(latex: string): number {
  let depth = 0;
  let max = 0;
  for (let i = 0; i < latex.length; i++) {
    const c = latex[i];
    if (c === '\\') {
      if (latex.startsWith('\\left', i) && !/[A-Za-z]/.test(latex[i + 5] ?? '')) {
        depth++;
        max = Math.max(max, depth);
        i += 4;
      } else if (latex.startsWith('\\right', i) && !/[A-Za-z]/.test(latex[i + 6] ?? '')) {
        depth = Math.max(0, depth - 1);
        i += 5;
      } else {
        i++; // skip the escaped character (\{, \}, \\)
      }
    } else if (c === '{') {
      depth++;
      max = Math.max(max, depth);
    } else if (c === '}') {
      depth = Math.max(0, depth - 1);
    }
  }
  return max;
}
