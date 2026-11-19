// renderMath (F053-AC-17, ADR-0055, research R1): LaTeX → monochrome SVG (`currentColor`, self-contained,
// `fontCache: 'none'`) + speech text, rendered in the browser with MathJax 4 (no server-side render worker). The
// LaTeX is checked first (length, nesting depth, forbidden commands); syntax errors are raised, never rendered as
// red error boxes. Results are cached by `math_id` = sha256(display + latex), so only changed formulas re-render.
// The server sanitises the SVG again and stores it as `math/<sha256>.svg` (F027).
import { mathjax } from '@mathjax/src/js/mathjax.js';
import { TeX } from '@mathjax/src/js/input/tex.js';
import { SVG } from '@mathjax/src/js/output/svg.js';
import { liteAdaptor, type LiteAdaptor } from '@mathjax/src/js/adaptors/liteAdaptor.js';
import { RegisterHTMLHandler } from '@mathjax/src/js/handlers/html.js';
import { STATE } from '@mathjax/src/js/core/MathItem.js';
import '@mathjax/src/js/input/tex/base/BaseConfiguration.js';
import '@mathjax/src/js/input/tex/ams/AmsConfiguration.js';
import '@mathjax/src/js/input/tex/mhchem/MhchemConfiguration.js';
import { MathJaxMhchemFontExtension } from '@mathjax/mathjax-mhchem-font-extension/js/svg.js';
import { forbiddenCommand, LATEX_MAX_DEPTH, LATEX_MAX_LENGTH, latexDepth, MATH_SVG_MAX, TEX_PACKAGES } from './mathjaxConfig';
import { speechText, type MmlLike, type SpeechLanguage } from './speech';

export type MathErrorCode = 'MATH_EMPTY' | 'MATH_TOO_LONG' | 'MATH_TOO_DEEP' | 'MATH_FORBIDDEN_COMMAND' | 'MATH_SYNTAX' | 'MATH_SVG_TOO_LARGE';

export class MathRenderError extends Error {
  readonly code: MathErrorCode;
  readonly latex: string;
  /** Offending command or MathJax message (shown next to the formula, never logged with content). */
  readonly detail?: string;

  constructor(code: MathErrorCode, latex: string, detail?: string) {
    super(code);
    this.name = 'MathRenderError';
    this.code = code;
    this.latex = latex;
    this.detail = detail;
  }
}

export interface RenderedMath {
  /** sha256 hex of `display + latex` (client cache key, ≤ 64). */
  math_id: string;
  latex: string;
  display: boolean;
  /** `<svg …>` with `currentColor` fills only. */
  svg: string;
  speech_text: string;
}

export interface RenderOptions {
  display?: boolean;
  /** Language of the speech text (content_language of the field). */
  language?: SpeechLanguage;
}

/** Pre-render checks shared with the editor (red highlight before saving). */
export function checkLatex(latex: string): MathRenderError | null {
  if (!latex.trim()) return new MathRenderError('MATH_EMPTY', latex);
  if (latex.length > LATEX_MAX_LENGTH) return new MathRenderError('MATH_TOO_LONG', latex, String(LATEX_MAX_LENGTH));
  if (latexDepth(latex) > LATEX_MAX_DEPTH) return new MathRenderError('MATH_TOO_DEEP', latex, String(LATEX_MAX_DEPTH));
  const bad = forbiddenCommand(latex);
  if (bad) return new MathRenderError('MATH_FORBIDDEN_COMMAND', latex, bad);
  return null;
}

/** Dynamic font ranges of mathjax-newcm (loaded on demand, bundled as separate chunks; no CDN URL — AGENTS R3). */
const FONT_RANGES: Record<string, () => Promise<unknown>> = {
  'accents': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/accents.js'),
  'accents-b-i': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/accents-b-i.js'),
  'arrows': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/arrows.js'),
  'marrows': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/marrows.js'),
  'calligraphic': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/calligraphic.js'),
  'double-struck': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/double-struck.js'),
  'fraktur': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/fraktur.js'),
  'greek': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/greek.js'),
  'latin': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/latin.js'),
  'latin-b': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/latin-b.js'),
  'latin-i': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/latin-i.js'),
  'latin-bi': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/latin-bi.js'),
  'math': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/math.js'),
  'mshapes': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/mshapes.js'),
  'shapes': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/shapes.js'),
  'symbols': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/symbols.js'),
  'symbols-b-i': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/symbols-b-i.js'),
  'script': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/script.js'),
  'variants': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/variants.js'),
  'sans-serif': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/sans-serif.js'),
  'sans-serif-b': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/sans-serif-b.js'),
  'sans-serif-i': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/sans-serif-i.js'),
  'sans-serif-bi': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/sans-serif-bi.js'),
  'sans-serif-r': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/sans-serif-r.js'),
  'sans-serif-ex': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/sans-serif-ex.js'),
  'monospace': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/monospace.js'),
  'monospace-l': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/monospace-l.js'),
  'monospace-ex': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/monospace-ex.js'),
  'PUA': () => import('@mathjax/mathjax-newcm-font/js/svg/dynamic/PUA.js'),
};

interface Engine {
  adaptor: LiteAdaptor;
  doc: ReturnType<typeof mathjax.document>;
}

let engine: Engine | null = null;

function getEngine(): Engine {
  if (engine) return engine;
  mathjax.asyncLoad = (name: string) => {
    const range = /dynamic\/([A-Za-z-]+)\.js$/.exec(name)?.[1];
    const load = range ? FONT_RANGES[range] : undefined;
    return load ? load() : Promise.reject(new Error(`font range not bundled: ${range ?? name}`));
  };
  const adaptor = liteAdaptor();
  RegisterHTMLHandler(adaptor);
  const tex = new TeX({
    packages: [...TEX_PACKAGES],
    // a syntax error is an exception, not a red "merror" box in the SVG
    formatError: (_jax: unknown, err: Error) => {
      throw err;
    },
  });
  const svg = new SVG({ fontCache: 'none', linebreaks: { inline: false } });
  // mhchem arrows (`->`, `<=>`) are glyphs of the mhchem font extension (MathJax 4)
  svg.font.addExtension(MathJaxMhchemFontExtension as never);
  engine = { adaptor, doc: mathjax.document('', { InputJax: tex, OutputJax: svg }) };
  return engine;
}

/** Forces every paint to `currentColor` (game tints the SVG with its theme, DESIGN_SYSTEM §5.2). */
export function monochrome(svg: string): string {
  return svg.replace(/\b(fill|stroke)="(?!none"|currentColor")[^"]*"/g, '$1="currentColor"').replace(/\bcolor="[^"]*"/g, '');
}

export async function sha256Hex(textValue: string): Promise<string> {
  const bytes = new TextEncoder().encode(textValue);
  const digest = await crypto.subtle.digest('SHA-256', bytes);
  return Array.from(new Uint8Array(digest), (b) => b.toString(16).padStart(2, '0')).join('');
}

export function mathId(latex: string, display: boolean): Promise<string> {
  return sha256Hex(`${display ? 'block' : 'inline'}:${latex}`);
}

const cache = new Map<string, Promise<RenderedMath>>();

async function renderUncached(latex: string, display: boolean, language: SpeechLanguage, id: string): Promise<RenderedMath> {
  const { adaptor, doc } = getEngine();
  let svg: string;
  let root: MmlLike;
  try {
    const node = await doc.convertPromise(latex, { display });
    const first = adaptor.firstChild(node);
    svg = monochrome(adaptor.outerHTML(first as Parameters<LiteAdaptor['outerHTML']>[0]));
    root = (await doc.convertPromise(latex, { display, end: STATE.COMPILED })) as unknown as MmlLike;
  } catch (err) {
    throw new MathRenderError('MATH_SYNTAX', latex, err instanceof Error ? err.message : undefined);
  }
  if (new TextEncoder().encode(svg).length > MATH_SVG_MAX) throw new MathRenderError('MATH_SVG_TOO_LARGE', latex, String(MATH_SVG_MAX));
  return { math_id: id, latex, display, svg, speech_text: speechText(root, language) };
}

/** Renders one formula (cached by math_id + language). Throws MathRenderError. */
export async function renderMath(latex: string, { display = false, language = 'vi' }: RenderOptions = {}): Promise<RenderedMath> {
  const bad = checkLatex(latex);
  if (bad) throw bad;
  const id = await mathId(latex, display);
  const key = `${id}:${language}`;
  let p = cache.get(key);
  if (!p) {
    p = renderUncached(latex, display, language, id);
    cache.set(key, p);
    p.catch(() => cache.delete(key));
  }
  return p;
}

/** Test helper: forget cached renders. */
export function clearMathCache(): void {
  cache.clear();
}
