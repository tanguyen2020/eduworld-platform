// renderImportMath (F054-AC-27, ADR-0055): before anything is sent, every formula of every row (JSON columns stem,
// render_payload, explanation, hints) is rendered in the browser with MathJax 4 (ops-ui renderMath:
// `latex_max_length`, `latex_max_depth`, `math_svg_max`, forbidden commands) and failures are reported by row and
// field. The rendered SVG + speech are written into the row's `math_assets` column (questions-row.schema.json) when
// it is empty, so the server stores the browser rendering [Proposed: instead of a ContentImportRequest.math_assets
// field, see Clarifications]. Pure apart from the MathJax call.
import { buildMathAssets, collectMath } from '@eduworld/ops-ui/content';
import { toCsv, type ImportTable } from './parseImportFile';

export const MATH_COLUMNS = ['stem', 'render_payload', 'explanation', 'hints'] as const;

export interface RowIssue {
  row: number;
  field: string;
  code: string;
  latex?: string;
}

export interface MathRenderResult {
  issues: RowIssue[];
  /** Rows holding at least one formula. */
  rowsWithMath: number;
  formulas: number;
  /** Normalized CSV to upload (with `math_assets`). */
  csv: string;
}

export async function renderImportMath(table: ImportTable, onProgress?: (done: number, total: number) => void): Promise<MathRenderResult> {
  const issues: RowIssue[] = [];
  const header = table.header.includes('math_assets') ? [...table.header] : [...table.header, 'math_assets'];
  const records: string[][] = [header];
  let rowsWithMath = 0;
  let formulas = 0;
  for (const [i, r] of table.rows.entries()) {
    const lang = r.cells.content_language === 'en' ? 'en' : 'vi';
    const payload: Record<string, unknown> = {};
    for (const col of MATH_COLUMNS) {
      const raw = r.cells[col];
      if (!raw) continue;
      try {
        payload[col] = JSON.parse(raw) as unknown;
      } catch {
        issues.push({ row: r.row, field: col, code: 'INVALID_JSON' });
      }
    }
    const found = collectMath(payload, lang);
    let assets = r.cells.math_assets ?? '';
    if (found.length) {
      rowsWithMath++;
      const built = await buildMathAssets(found);
      formulas += built.assets.length;
      for (const e of built.errors) issues.push({ row: r.row, field: e.path.split('.')[0] ?? '', code: e.error.code, latex: e.latex });
      if (!assets && built.errors.length === 0) assets = JSON.stringify(built.assets);
    }
    // keep the file's row numbers: skipped (empty) rows are written as empty records, which the server counts too
    while (records.length < r.row - 1) records.push(header.map(() => ''));
    records.push(header.map((h) => (h === 'math_assets' ? assets : (r.cells[h] ?? ''))));
    onProgress?.(i + 1, table.rows.length);
  }
  return { issues, rowsWithMath, formulas, csv: toCsv(records) };
}
