// F054 T054 (F054-AC-27, FR-QB-011): the browser reads CSV / XLSX into rows numbered like the server report, checks
// the required columns and the row cap, renders every formula with MathJax 4 and reports formula errors by row BEFORE
// sending; the uploaded CSV carries `math_assets` and keeps the row numbers.
import { describe, expect, it } from 'vitest';
import { columnIndex, ImportParseError, importKindOf, parseCsv, parseImportFile, readXlsx, REQUIRED_COLUMNS, toCsv, toTable } from './parseImportFile';
import { renderImportMath } from './renderImportMath';

const enc = new TextEncoder();

/** MOCK (test only): a stored (uncompressed) zip — enough for the XLSX reader. */
function zip(files: Record<string, string>): Uint8Array {
  const parts: number[] = [];
  const central: number[] = [];
  const u16 = (a: number[], v: number) => a.push(v & 0xff, (v >> 8) & 0xff);
  const u32 = (a: number[], v: number) => a.push(v & 0xff, (v >> 8) & 0xff, (v >> 16) & 0xff, (v >>> 24) & 0xff);
  for (const [name, text] of Object.entries(files)) {
    const data = enc.encode(text);
    const nm = enc.encode(name);
    const offset = parts.length;
    u32(parts, 0x04034b50); u16(parts, 20); u16(parts, 0); u16(parts, 0); u16(parts, 0); u16(parts, 0); u32(parts, 0); u32(parts, data.length); u32(parts, data.length); u16(parts, nm.length); u16(parts, 0);
    parts.push(...nm, ...data);
    u32(central, 0x02014b50); u16(central, 20); u16(central, 20); u16(central, 0); u16(central, 0); u16(central, 0); u16(central, 0); u32(central, 0); u32(central, data.length); u32(central, data.length); u16(central, nm.length); u16(central, 0); u16(central, 0); u16(central, 0); u16(central, 0); u32(central, 0); u32(central, offset);
    central.push(...nm);
  }
  const cdOffset = parts.length;
  const out = [...parts, ...central];
  u32(out, 0x06054b50); u16(out, 0); u16(out, 0); u16(out, Object.keys(files).length); u16(out, Object.keys(files).length); u32(out, central.length); u32(out, cdOffset); u16(out, 0);
  return Uint8Array.from(out);
}

const header = [...REQUIRED_COLUMNS, 'content_language'];
const stem = (text: string) => JSON.stringify({ blocks: [{ type: 'paragraph', text: { vi: text } }] });
const row = (s: string) => ['single_choice', 'MATH', '3', 'CT-2018', 'T1', 'LO1', '2', s, '{"options":[]}', '{"correct_option_id":"o_a"}', '{"mode":"all_or_nothing"}', 'vi'];

describe('parseImportFile', () => {
  it('CSV: quotes, doubled quotes, newlines in quotes, BOM; toCsv round trip', () => {
    const text = '﻿a,b\r\n"x, y","say ""hi"""\n"multi\nline",2\n';
    const recs = parseCsv(text);
    expect(recs).toEqual([['a', 'b'], ['x, y', 'say "hi"'], ['multi\nline', '2']]);
    expect(parseCsv(toCsv(recs))).toEqual(recs);
    expect(() => parseCsv('"open')).toThrow(ImportParseError);
  });

  it('table: header lower-cased, missing column, no data, row numbers kept (header = 1), empty rows skipped', () => {
    expect(() => toTable([['type'], ['x']])).toThrow(/MISSING_COLUMN/);
    expect(() => toTable([header])).toThrow(/NO_DATA/);
    const tb = toTable([header.map((h) => h.toUpperCase()), row('a'), [], row('b')]);
    expect(tb.rows.map((r) => r.row)).toEqual([2, 4]);
    expect(tb.rows[1]!.cells.stem).toBe('b');
    expect(importKindOf('Cau_hoi.XLSX')).toBe('questions_xlsx');
    expect(importKindOf('a.docx')).toBeNull();
  });

  it('XLSX: first sheet through the workbook relationships, shared + inline strings, gaps', async () => {
    const bytes = zip({
      'xl/workbook.xml': '<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Q" sheetId="1" r:id="rId9"/></sheets></workbook>',
      'xl/_rels/workbook.xml.rels': '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId9" Target="worksheets/data.xml"/></Relationships>',
      'xl/sharedStrings.xml': '<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><si><t>type</t></si><si><r><t>sub</t></r><r><t>ject_code</t></r></si></sst>',
      'xl/worksheets/data.xml':
        '<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="s"><v>0</v></c><c r="C1" t="s"><v>1</v></c></row><row r="3"><c r="A3" t="inlineStr"><is><t>numeric</t></is></c><c r="B3"><v>42</v></c></row></sheetData></worksheet>',
    });
    expect(await readXlsx(bytes)).toEqual([['type', '', 'subject_code'], [], ['numeric', '42']]);
    expect(columnIndex('AB12')).toBe(27);
    const file = new File([(zip({ 'xl/worksheets/sheet1.xml': `<worksheet><sheetData><row r="1">${header.map((h, i) => `<c r="${String.fromCharCode(65 + i)}1" t="inlineStr"><is><t>${h}</t></is></c>`).join('')}</row><row r="2">${row('x').map((v, i) => `<c r="${String.fromCharCode(65 + i)}2" t="inlineStr"><is><t>${v.replace(/</g, '&lt;').replace(/&/g, '&amp;')}</t></is></c>`).join('')}</row></sheetData></worksheet>` }) as BlobPart)], 'q.xlsx');
    const tb = await parseImportFile(file);
    expect(tb.rows).toHaveLength(1);
    expect(tb.rows[0]!.cells.type).toBe('single_choice');
  });

  it('formulas: errors by row and field before sending; math_assets filled; row numbers kept in the CSV', async () => {
    const tb = toTable([header, row(stem('Tính \\(\\frac{1}{2} + 1\\)')), [], row(stem('Lỗi \\(\\href{x}{y}\\)')), row('{bad json'), row(stem('Không có công thức'))]);
    const r = await renderImportMath(tb);
    expect(r.issues).toEqual([
      expect.objectContaining({ row: 4, field: 'stem', code: 'MATH_FORBIDDEN_COMMAND' }),
      expect.objectContaining({ row: 5, field: 'stem', code: 'INVALID_JSON' }),
    ]);
    expect(r.rowsWithMath).toBe(2);
    const out = parseCsv(r.csv);
    expect(out[0]!.at(-1)).toBe('math_assets');
    expect(out).toHaveLength(6);
    expect(out[2]!.every((c) => c === '')).toBe(true); // empty row 3 kept so the server numbers rows like the file
    const assets = JSON.parse(out[1]!.at(-1)!) as { latex: string; svg: string; speech_text: string }[];
    expect(assets[0]!.latex).toBe('\\frac{1}{2} + 1');
    expect(assets[0]!.svg).toContain('<svg');
    expect(out[3]!.at(-1)).toBe(''); // a failing row gets no assets
  });
});
