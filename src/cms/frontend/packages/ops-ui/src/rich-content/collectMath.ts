// collectMath (F053-AC-17, AC-18, F053 T038): finds every formula of an authoring payload — delimited LaTeX in text
// strings and `math` blocks (`{type: 'math', latex}`) — renders each once in the browser (MathJax 4) and builds
// `math_assets[]` ({latex, svg, speech_text}) for the save request. Formula errors are returned with the path of the
// field holding them (red highlight + blocker); a server `422 MATH_RENDER_FAILED` (`/math_assets/<i>/…`) maps back to
// the same paths through `assetPaths`.
import { splitMath } from '../math/delimiters';
import { renderMath, MathRenderError, type RenderedMath } from '../math/renderMath';
import type { SpeechLanguage } from '../math/speech';

export interface FoundMath {
  latex: string;
  display: boolean;
  /** Dotted path of the field holding the formula, e.g. `stem.blocks.0.text.vi`. */
  path: string;
  language: SpeechLanguage;
}

export interface MathAssetInput {
  latex: string;
  svg: string;
  speech_text: string;
}

export interface MathBuildResult {
  assets: MathAssetInput[];
  /** For each asset index, the field paths using that formula. */
  assetPaths: string[][];
  errors: { path: string; latex: string; error: MathRenderError }[];
}

function walk(node: unknown, path: string, lang: SpeechLanguage, out: FoundMath[]): void {
  if (typeof node === 'string') {
    for (const s of splitMath(node)) if (s.kind === 'math') out.push({ latex: s.latex, display: s.display, path, language: lang });
    return;
  }
  if (Array.isArray(node)) {
    node.forEach((x, i) => walk(x, path ? `${path}.${i}` : String(i), lang, out));
    return;
  }
  if (node && typeof node === 'object') {
    const o = node as Record<string, unknown>;
    if (o.type === 'math' && typeof o.latex === 'string' && o.latex.trim()) out.push({ latex: o.latex.trim(), display: true, path: path ? `${path}.latex` : 'latex', language: lang });
    for (const [k, v] of Object.entries(o)) {
      if (k === 'latex' || k === 'math_assets') continue;
      const childLang = k === 'vi' || k === 'en' ? k : lang;
      walk(v, path ? `${path}.${k}` : k, childLang, out);
    }
  }
}

/** Every formula of a payload (fields `vi`/`en` give the speech language; default = content language). */
export function collectMath(payload: unknown, contentLanguage: SpeechLanguage): FoundMath[] {
  const out: FoundMath[] = [];
  walk(payload, '', contentLanguage, out);
  return out;
}

/** Renders the formulas (one asset per distinct LaTeX) and reports the failing ones by field. */
export async function buildMathAssets(found: readonly FoundMath[]): Promise<MathBuildResult> {
  const byLatex = new Map<string, FoundMath[]>();
  for (const f of found) byLatex.set(f.latex, [...(byLatex.get(f.latex) ?? []), f]);
  const assets: MathAssetInput[] = [];
  const assetPaths: string[][] = [];
  const errors: MathBuildResult['errors'] = [];
  for (const [latex, uses] of byLatex) {
    const first = uses.find((u) => !u.display) ?? uses[0]!;
    try {
      const r: RenderedMath = await renderMath(latex, { display: first.display, language: first.language });
      assets.push({ latex, svg: r.svg, speech_text: r.speech_text });
      assetPaths.push(uses.map((u) => u.path));
    } catch (e) {
      const err = e instanceof MathRenderError ? e : new MathRenderError('MATH_SYNTAX', latex);
      for (const u of uses) errors.push({ path: u.path, latex, error: err });
    }
  }
  return { assets, assetPaths, errors };
}

/** Field paths of a server `MATH_RENDER_FAILED` pointer (`/math_assets/3/latex` → paths of asset 3). */
export function pathsForMathPointer(pointer: string, assetPaths: readonly string[][]): string[] {
  const m = /^\/math_assets\/(\d+)/.exec(pointer);
  return m ? (assetPaths[Number(m[1])] ?? []) : [];
}
