// renderDocxFormulas (F028-AC-15, ADR-0055 §4): the formulas the server found in one question of a Word import
// (`ContentImportItem.formulas`, LaTeX already through the allowlist) are rendered in the browser with MathJax 4 (ops-ui
// renderMath: SVG + speech text) into the `math_assets` of createDraftFromImportItem. A formula that does not render
// is reported with its code and nothing is sent for that question. Pure apart from the MathJax call.
import { MathRenderError, renderMath } from '@eduworld/ops-ui/content';

export interface DocxFormula {
  latex: string;
  display: boolean;
}

export interface DocxMathAsset {
  latex: string;
  svg: string;
  speech_text: string;
}

export interface DocxMathResult {
  assets: DocxMathAsset[];
  /** Code of the first formula that failed (MathErrorCode), or null. */
  error: string | null;
}

export async function renderDocxFormulas(formulas: readonly DocxFormula[], language: 'vi' | 'en'): Promise<DocxMathResult> {
  const assets: DocxMathAsset[] = [];
  for (const f of formulas) {
    try {
      const r = await renderMath(f.latex, { display: f.display, language });
      assets.push({ latex: f.latex, svg: r.svg, speech_text: r.speech_text });
    } catch (e) {
      return { assets: [], error: e instanceof MathRenderError ? e.code : 'MATH_SYNTAX' };
    }
  }
  return { assets, error: null };
}
